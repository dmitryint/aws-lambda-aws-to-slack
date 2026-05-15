package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"golang.org/x/sync/errgroup"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/config"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	autoscalingparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/autoscaling"
	awshealthparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/awshealth"
	batchparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/batch"
	beanstalkparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/beanstalk"
	cloudformationparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/cloudformation"
	cloudwatchparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/cloudwatch"
	codebuildparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/codebuild"
	codecommitparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/codecommit"
	codedeployparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/codedeploy"
	codepipelineparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/codepipeline"
	ecsparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/ecs"
	genericparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/generic"
	guarddutyparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/guardduty"
	inspectorparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/inspector"
	inspector2parser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/inspector2"
	rdsparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/rds"
	sesparser "github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser/ses"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/router"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/transport"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/transport/slack"
)

// maxInFlight bounds how many notification deliveries run concurrently
// when an SNS multi-record event is fanned out. A small fan-out keeps
// burst rates below Slack's 429 threshold while still completing well
// within the Lambda deadline.
const maxInFlight = 4

// Handler wires the envelope → router → transport pipeline.
//
// One Handler is constructed once at cold start and reused for the lifetime
// of the Lambda container. The struct holds no mutable state.
type Handler struct {
	cfg       *config.Config
	awsCfg    aws.Config
	router    *router.Router
	renderers []transport.Renderer
	log       *slog.Logger
}

// Option configures the Handler at construction time.
type Option func(*Handler)

// WithRenderers replaces the default transport list. Tests inject a
// recording renderer; production wires the Slack renderer.
func WithRenderers(renderers ...transport.Renderer) Option {
	return func(h *Handler) { h.renderers = renderers }
}

// WithLogger overrides the default slog logger (slog.Default()).
func WithLogger(l *slog.Logger) Option { return func(h *Handler) { h.log = l } }

// WithRouter overrides the default router. Tests use this to register
// custom parsers without going through the production wiring.
func WithRouter(r *router.Router) Option { return func(h *Handler) { h.router = r } }

// New returns a Handler with the default router (every wave's parsers plus
// the generic catch-all) and the Slack transport renderer wired from the
// provided Config.
func New(cfg *config.Config, awsCfg aws.Config, opts ...Option) *Handler {
	h := &Handler{
		cfg:    cfg,
		awsCfg: awsCfg,
		router: defaultRouter(cfg, awsCfg),
		renderers: []transport.Renderer{
			slack.NewRenderer(slack.New(cfg.SlackHookURL)),
		},
		log: slog.Default(),
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// defaultRouter builds the production parser waterfall (first match wins);
// generic is always last so every event produces some output.
//
// Parsers that wrap SDK clients (codecommit-repository, cloudwatch,
// inspector2) are constructed from the cold-start aws.Config so every cold
// start gets a fresh client. SDK calls never happen at boot.
func defaultRouter(cfg *config.Config, awsCfg aws.Config) *router.Router {
	chartCfg := cloudwatchparser.ChartConfig{
		BucketName:     cfg.ChartBucketName,
		BucketRegion:   cfg.ChartBucketRegion,
		FallbackRegion: cfg.Region,
		URLTTL:         time.Duration(cfg.ChartURLTTLDays) * 24 * time.Hour,
		SSEAlgorithm:   cfg.ChartBucketSSE,
	}
	r := router.New()
	r.Register(autoscalingparser.New())
	r.Register(awshealthparser.New())
	r.Register(batchparser.New())
	r.Register(beanstalkparser.New())
	r.Register(cloudformationparser.New())
	r.Register(cloudwatchparser.NewFromConfig(awsCfg, chartCfg))
	r.Register(codebuildparser.New())
	r.Register(codecommitparser.NewPullRequest())
	r.Register(codecommitparser.NewRepositoryFromConfig(awsCfg))
	r.Register(codedeployparser.NewCloudWatch())
	r.Register(codedeployparser.NewSNS())
	r.Register(codepipelineparser.New())
	r.Register(codepipelineparser.NewApproval())
	r.Register(guarddutyparser.New())
	r.Register(inspectorparser.New())
	r.Register(inspector2parser.NewFromConfig(awsCfg, cfg.DedupTableName, cfg.DedupTTLDays))
	r.Register(rdsparser.New())
	r.Register(ecsparser.New())
	r.Register(sesparser.NewBounce())
	r.Register(sesparser.NewComplaint())
	r.Register(sesparser.NewReceived())
	r.Register(genericparser.New())
	return r
}

// Handle is the Lambda entrypoint. It accepts any JSON payload (SNS,
// EventBridge, direct), fans SNS multi-record payloads out into per-record
// envelopes, routes each through the parser waterfall, and posts the
// resulting Slack message.
//
// Records are processed concurrently with a fan-out bound. Partial
// failures continue — the handler aggregates errors and returns a joined
// error so the Lambda runtime increments the Errors metric for any record
// that failed.
func (h *Handler) Handle(ctx context.Context, raw json.RawMessage) error {
	if isEmptySNSRecords(raw) {
		return nil
	}
	ev, err := envelope.New(raw)
	if err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}
	records := ev.Records()
	if len(records) == 0 {
		return nil
	}

	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(maxInFlight)
	var (
		errsMu sync.Mutex
		errs   []error
	)

	for _, rec := range records {
		group.Go(func() error {
			if perr := h.processRecord(gctx, rec); perr != nil {
				errsMu.Lock()
				errs = append(errs, perr)
				errsMu.Unlock()
				h.log.ErrorContext(gctx, "record processing failed",
					"err", perr,
					"source", rec.Source(),
					"subject", rec.Subject(),
				)
			}
			return nil
		})
	}
	if werr := group.Wait(); werr != nil {
		return fmt.Errorf("handler wait: %w", werr)
	}
	if len(errs) > 0 {
		return fmt.Errorf("handler: %d records failed: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// isEmptySNSRecords detects the explicit "SNS envelope with zero records"
// payload. An SNS-shaped envelope with an empty Records array is treated
// as a no-op (no parsers run, no Slack posts) — distinct from a non-SNS
// direct invocation, which produces a single event.
func isEmptySNSRecords(raw json.RawMessage) bool {
	var outer struct {
		Records *[]json.RawMessage `json:"Records"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return false
	}
	return outer.Records != nil && len(*outer.Records) == 0
}

// processRecord drives the router and fan-out to every accepting transport
// renderer for a single record. Per-transport errors are aggregated; the
// loop continues so one failing transport never silences the others.
func (h *Handler) processRecord(ctx context.Context, rec *envelope.Event) error {
	n, err := h.router.Route(ctx, rec)
	if err != nil {
		return fmt.Errorf("route: %w", err)
	}
	if n == nil {
		h.log.InfoContext(ctx, "record silenced",
			"source", rec.Source(),
			"subject", rec.Subject(),
			"detail_type", rec.DetailType(),
		)
		return nil
	}

	var errs []error
	delivered := 0
	for _, r := range h.renderers {
		if !r.Accepts(n.Severity) {
			continue
		}
		if perr := r.Send(ctx, n); perr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name(), perr))
			continue
		}
		delivered++
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	h.log.InfoContext(ctx, "alert delivered",
		"source", rec.Source(),
		"severity", n.Severity.String(),
		"transports", delivered,
	)
	return nil
}
