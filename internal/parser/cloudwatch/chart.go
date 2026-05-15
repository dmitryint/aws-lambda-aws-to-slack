package cloudwatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	defaultChartURLTTL = 7 * 24 * time.Hour
	chartWidgetWidth   = 600
	chartWidgetHeight  = 300
	chartWidgetStart   = "-PT3H"
	chartWidgetEnd     = "P0D"

	chartContentType = "image/png"

	// uuidLen is the byte length used to build the random object key.
	uuidLen = 16

	hoursPerLambdaLog = 1
)

// regionNameToID maps the human-readable region name CloudWatch publishes
// in the alarm payload to its canonical region id, which the chart and
// console URLs need.
var regionNameToID = map[string]string{
	"US East (Ohio)":             "us-east-2",
	"US East (N. Virginia)":      "us-east-1",
	"US West (N. California)":    "us-west-1",
	"US West (Oregon)":           "us-west-2",
	"Asia Pacific (Mumbai)":      "ap-south-1",
	"Asia Pacific (Osaka-Local)": "ap-northeast-3",
	"Asia Pacific (Seoul)":       "ap-northeast-2",
	"Asia Pacific (Singapore)":   "ap-southeast-1",
	"Asia Pacific (Sydney)":      "ap-southeast-2",
	"Asia Pacific (Tokyo)":       "ap-northeast-1",
	"Canada (Central)":           "ca-central-1",
	"China (Beijing)":            "cn-north-1",
	"China (Ningxia)":            "cn-northwest-1",
	"EU (Frankfurt)":             "eu-central-1",
	"EU (Ireland)":               "eu-west-1",
	"EU (London)":                "eu-west-2",
	"EU (Paris)":                 "eu-west-3",
	"EU (Stockholm)":             "eu-north-1",
	"South America (São Paulo)":  "sa-east-1",
	"South America (Sao Paulo)":  "sa-east-1",
	"AWS GovCloud (US-East)":     "us-gov-east-1",
	"AWS GovCloud (US)":          "us-gov-west-1",
}

// resolveRegion returns the canonical region id for a CloudWatch alarm
// `Region` field. Returns the fallback when the name is not in the map.
func resolveRegion(regionName, fallback string) string {
	if id, ok := regionNameToID[regionName]; ok {
		return id
	}
	return fallback
}

// ChartRenderer is the seam tests use to inject a fake CloudWatch SDK
// client.
type ChartRenderer interface {
	GetMetricWidgetImage(ctx context.Context, in *cloudwatch.GetMetricWidgetImageInput,
		optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricWidgetImageOutput, error)
}

// ChartUploader is the seam tests use to inject a fake S3 PutObject client.
type ChartUploader interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput,
		optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// PresignedRequest is the subset of v4.PresignedHTTPRequest the chart
// rendering path returns to its caller. Keeping the interface narrow lets the
// tests avoid pulling the aws-sdk presign package transitively.
type PresignedRequest struct {
	URL    string
	Method string
}

// Presigner is the seam tests use to inject a fake S3 presigner. The
// presigner returns a signed GET URL that Slack can embed in an image
// block.
type Presigner interface {
	PresignGetObject(ctx context.Context, in *s3.GetObjectInput,
		optFns ...func(*s3.PresignOptions)) (*PresignedRequest, error)
}

// ChartConfig bundles the cold-start config the renderer needs.
type ChartConfig struct {
	BucketName   string
	BucketRegion string
	// FallbackRegion is used when the alarm's Region field cannot be mapped
	// to a canonical region id via regionNameToID.
	FallbackRegion string
	// URLTTL is the presigned-URL Expires value passed to s3.PresignGetObject.
	// Zero falls back to defaultChartURLTTL.
	URLTTL time.Duration
	// SSEAlgorithm is the value sent as x-amz-server-side-encryption on the
	// chart upload. Operators set CHART_BUCKET_SSE to "aws:kms" (default),
	// "AES256", or an empty string to omit the header entirely. The latter
	// only works when the bucket policy does not require encrypted uploads.
	SSEAlgorithm string
}

// ChartRenderingPipeline bundles the three SDK seams required to render and
// host a chart for a CloudWatch alarm.
type ChartRenderingPipeline struct {
	Renderer  ChartRenderer
	Uploader  ChartUploader
	Presigner Presigner
	Config    ChartConfig
	Logger    *slog.Logger
}

// renderAlarmChart runs the full GetMetricWidgetImage → PutObject → Presign
// pipeline. Every error path is logged at slog.Error and returned to the
// caller as an empty URL — the parser then renders the alert without an
// image block.
//
// When BucketName is empty the function returns ("", nil) — chart rendering
// is intentionally disabled.
func (p *ChartRenderingPipeline) renderAlarmChart(ctx context.Context, m alarmMessage) string {
	if p == nil || p.Config.BucketName == "" {
		return ""
	}
	if p.Renderer == nil || p.Uploader == nil || p.Presigner == nil {
		return ""
	}

	region := resolveRegion(m.Region, p.Config.FallbackRegion)
	widgetJSON, err := buildWidgetJSON(m, region)
	if err != nil {
		p.logger().Error("cloudwatch chart widget build failed",
			"err", err,
			"alarm", m.AlarmName,
		)
		return ""
	}

	img, err := p.Renderer.GetMetricWidgetImage(ctx, &cloudwatch.GetMetricWidgetImageInput{
		MetricWidget: aws.String(string(widgetJSON)),
	})
	if err != nil {
		p.logger().Error("cloudwatch chart GetMetricWidgetImage failed",
			"err", err,
			"alarm", m.AlarmName,
		)
		return ""
	}

	key, err := buildChartKey(time.Now().UTC())
	if err != nil {
		p.logger().Error("cloudwatch chart key build failed",
			"err", err,
			"alarm", m.AlarmName,
		)
		return ""
	}

	putIn := &s3.PutObjectInput{
		Bucket:      aws.String(p.Config.BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(img.MetricWidgetImage),
		ContentType: aws.String(chartContentType),
	}
	if p.Config.SSEAlgorithm != "" {
		putIn.ServerSideEncryption = s3types.ServerSideEncryption(p.Config.SSEAlgorithm)
	}
	if _, err := p.Uploader.PutObject(ctx, putIn); err != nil {
		p.logger().Error("cloudwatch chart PutObject failed",
			"err", err,
			"alarm", m.AlarmName,
			"bucket", p.Config.BucketName,
		)
		return ""
	}

	ttl := p.Config.URLTTL
	if ttl <= 0 {
		ttl = defaultChartURLTTL
	}
	signed, err := p.Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.Config.BucketName),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		p.logger().Error("cloudwatch chart PresignGetObject failed",
			"err", err,
			"alarm", m.AlarmName,
			"bucket", p.Config.BucketName,
		)
		return ""
	}
	return signed.URL
}

// logger returns the configured slog logger, falling back to slog.Default().
func (p *ChartRenderingPipeline) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

// widgetMetricEntry is one element of the `metrics` array in the widget
// JSON document. The JSON shape is a heterogeneous array — strings followed
// by an options object — so we marshal it as []any.
type widgetMetricEntry []any

// widget is the JSON the CloudWatch GetMetricWidgetImage API consumes.
type widget struct {
	Title       string             `json:"title,omitempty"`
	Width       int                `json:"width"`
	Height      int                `json:"height"`
	Start       string             `json:"start"`
	End         string             `json:"end"`
	Region      string             `json:"region"`
	Annotations *widgetAnnotations `json:"annotations,omitempty"`
	Metrics     [][]any            `json:"metrics,omitempty"`
}

// widgetAnnotations carries the threshold horizontal line annotation.
type widgetAnnotations struct {
	Horizontal []widgetHorizontalAnnotation `json:"horizontal"`
}

// widgetHorizontalAnnotation is one horizontal-line entry.
type widgetHorizontalAnnotation struct {
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

// buildWidgetJSON serializes the widget document the chart API expects.
//
// When Trigger.Metrics is empty it falls back to Trigger.Dimensions; when
// Trigger.Metrics is present and a MetricStat is configured, it prefers
// MetricStat.Metric.Dimensions over Trigger.Dimensions so metric-math
// alarms render the metric they actually evaluate against.
func buildWidgetJSON(m alarmMessage, region string) (json.RawMessage, error) {
	w := widget{
		Title:  m.AlarmName,
		Width:  chartWidgetWidth,
		Height: chartWidgetHeight,
		Start:  chartWidgetStart,
		End:    chartWidgetEnd,
		Region: region,
	}
	if m.Trigger.Threshold != nil {
		w.Annotations = &widgetAnnotations{
			Horizontal: []widgetHorizontalAnnotation{{
				Value: *m.Trigger.Threshold,
				Label: "threshold",
			}},
		}
	}
	if len(m.Trigger.Metrics) > 0 {
		entries := make([][]any, 0, len(m.Trigger.Metrics))
		for _, metric := range m.Trigger.Metrics {
			entries = append(entries, buildMetricEntry(metric))
		}
		w.Metrics = entries
	} else {
		w.Metrics = [][]any{buildSimpleMetricEntry(m.Trigger)}
	}
	return json.Marshal(w)
}

// buildMetricEntry constructs the widget metric entry for a Trigger.Metrics
// element. The renderer falls back to MetricStat.Metric.Dimensions when
// Trigger.Dimensions is empty.
func buildMetricEntry(metric metricEntry) widgetMetricEntry {
	if metric.MetricStat != nil {
		dims := metric.MetricStat.Metric.Dimensions
		const baseLen = 2
		const optsLen = 1
		out := make(widgetMetricEntry, 0, baseLen+2*len(dims)+optsLen)
		out = append(out,
			metric.MetricStat.Metric.Namespace,
			metric.MetricStat.Metric.MetricName,
		)
		for _, d := range dims {
			out = append(out, d.Name, d.Value)
		}
		opts := map[string]any{
			"id":      metric.ID,
			"stat":    metric.MetricStat.Stat,
			"period":  metric.MetricStat.Period,
			"visible": metric.ReturnData == nil || *metric.ReturnData,
		}
		out = append(out, opts)
		return out
	}
	opts := map[string]any{
		"id":      metric.ID,
		"visible": metric.ReturnData == nil || *metric.ReturnData,
	}
	if metric.Expression != "" {
		opts["expression"] = metric.Expression
	}
	label := metric.Label
	if label == "" {
		label = metric.ID
	}
	opts["label"] = label
	return widgetMetricEntry{opts}
}

// buildSimpleMetricEntry constructs the widget metric entry for an alarm
// without Trigger.Metrics — the simple-metric path.
func buildSimpleMetricEntry(tr trigger) widgetMetricEntry {
	const baseLen = 2
	const optsLen = 1
	out := make(widgetMetricEntry, 0, baseLen+2*len(tr.Dimensions)+optsLen)
	out = append(out, tr.Namespace, tr.MetricName)
	for _, d := range tr.Dimensions {
		out = append(out, d.Name, d.Value)
	}
	out = append(out, map[string]any{
		"stat":   upperFirstLowerRest(tr.Statistic),
		"period": tr.Period,
	})
	return out
}

// upperFirstLowerRest mirrors lodash's `_.upperFirst(_.toLower(str))`.
func upperFirstLowerRest(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

// buildChartKey returns the S3 object key for a new chart upload — a
// per-day prefix plus a random suffix: `charts/${date}/${uuid}.png`.
func buildChartKey(now time.Time) (string, error) {
	buf := make([]byte, uuidLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	suffix := hex.EncodeToString(buf)
	datePrefix := now.Format("2006-01-02")
	return fmt.Sprintf("charts/%s/%s.png", datePrefix, suffix), nil
}

// metricsLogsURL builds the CloudWatch logs URL for the Lambda function in
// the alarm trigger, returning empty when the trigger is not an AWS/Lambda
// Errors-style alarm with a FunctionName dimension.
func metricsLogsURL(tr trigger, ts time.Time, region string) string {
	if tr.Namespace != "AWS/Lambda" {
		return ""
	}
	var fn string
	for _, d := range tr.Dimensions {
		if d.Name == "FunctionName" {
			fn = d.Value
			break
		}
	}
	if fn == "" {
		return ""
	}
	eventTime := ts.UTC().Truncate(time.Hour)
	start := eventTime.Add(-hoursPerLambdaLog * time.Hour)
	end := eventTime.Add(hoursPerLambdaLog * time.Hour)
	logGroup := "/aws/lambda/" + fn

	var b strings.Builder
	b.WriteString("https://console.aws.amazon.com/cloudwatch/home?")
	if region != "" {
		b.WriteString("region=" + region)
	}
	b.WriteString("#logEventViewer:group=" + url.QueryEscape(logGroup))
	b.WriteString(";start=" + start.Format(time.RFC3339))
	b.WriteString(";end=" + end.Format(time.RFC3339))
	return b.String()
}
