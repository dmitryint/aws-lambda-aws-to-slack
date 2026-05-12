package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/config"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/handler"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/kms"
)

// revision is overridden at link time by the Makefile via
// -ldflags "-X main.revision=...". The structured logger surfaces it on
// every log line so a misbehaving Lambda is traceable back to a commit.
var revision = "dev"

// coldStartTimeout caps the time spent on AWS config + parallel KMS
// decrypts during init. A hung KMS endpoint surfaces as a timeout error
// rather than a silent hang.
const coldStartTimeout = 5 * time.Second

func main() { os.Exit(execute(context.Background(), productionDeps())) }

// execute is the testable wrapper around run + the os.Exit decision. It
// returns the process exit code rather than calling os.Exit directly so
// the cold-start fail-closed contract can be asserted in tests.
func execute(ctx context.Context, deps runDeps) int {
	if err := run(ctx, deps); err != nil {
		slog.Error("cold-start failed", "err", err)
		return 1
	}
	return 0
}

// runDeps is the set of seams main exercises in production but tests
// override. awsLoader returns the SDK configuration; kmsFactory wraps
// that configuration into the Decrypter used by config.Load; starter
// hands the wired handler off to the Lambda runtime.
type runDeps struct {
	awsLoader  func(ctx context.Context) (aws.Config, error)
	kmsFactory func(aws.Config) kms.Decrypter
	starter    func(h *handler.Handler)
}

// productionDeps returns the production wiring. Exposed (lowercased) so
// main_test.go can assert all three seams are non-nil.
func productionDeps() runDeps {
	return runDeps{
		awsLoader: func(ctx context.Context) (aws.Config, error) {
			return awsconfig.LoadDefaultConfig(ctx)
		},
		kmsFactory: func(c aws.Config) kms.Decrypter {
			return kms.NewClient(c)
		},
		starter: func(h *handler.Handler) { lambda.Start(h.Handle) },
	}
}

// run is the testable cold-start path. It is split out of main() so the
// fail-closed contract can be asserted without invoking os.Exit. The
// caller is responsible for surfacing a non-nil return as the process
// exit code (main() does that; tests assert it).
func run(ctx context.Context, deps runDeps) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("revision", revision)
	slog.SetDefault(logger)

	initCtx, cancel := context.WithTimeout(ctx, coldStartTimeout)
	defer cancel()

	awsCfg, err := deps.awsLoader(initCtx)
	if err != nil {
		return fmt.Errorf("aws config load: %w", err)
	}

	kmsClient := deps.kmsFactory(awsCfg)
	cfg, err := config.Load(initCtx, kmsClient)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}

	h := handler.New(cfg, awsCfg)
	deps.starter(h)
	return nil
}
