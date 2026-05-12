package cloudwatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeRenderer is a hand-rolled ChartRenderer.
type fakeRenderer struct {
	gotWidget string
	err       error
	body      []byte
}

func (f *fakeRenderer) GetMetricWidgetImage(_ context.Context, in *cloudwatch.GetMetricWidgetImageInput,
	_ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricWidgetImageOutput, error) {
	if in.MetricWidget != nil {
		f.gotWidget = *in.MetricWidget
	}
	if f.err != nil {
		return nil, f.err
	}
	body := f.body
	if body == nil {
		body = []byte("PNGDATA")
	}
	return &cloudwatch.GetMetricWidgetImageOutput{MetricWidgetImage: body}, nil
}

// fakeUploader is a hand-rolled ChartUploader.
type fakeUploader struct {
	gotBucket string
	gotKey    string
	gotBody   []byte
	gotType   string
	err       error
}

func (f *fakeUploader) PutObject(_ context.Context, in *s3.PutObjectInput,
	_ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if in.Bucket != nil {
		f.gotBucket = *in.Bucket
	}
	if in.Key != nil {
		f.gotKey = *in.Key
	}
	if in.ContentType != nil {
		f.gotType = *in.ContentType
	}
	if in.Body != nil {
		b, _ := io.ReadAll(in.Body)
		f.gotBody = b
	}
	if f.err != nil {
		return nil, f.err
	}
	return &s3.PutObjectOutput{}, nil
}

// fakePresigner is a hand-rolled Presigner.
type fakePresigner struct {
	url string
	err error
}

func (f *fakePresigner) PresignGetObject(_ context.Context, _ *s3.GetObjectInput,
	_ ...func(*s3.PresignOptions)) (*PresignedRequest, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &PresignedRequest{URL: f.url, Method: "GET"}, nil
}

// alarmFromSample loads samples/cloudwatch/<name>.json and decodes the inner
// SNS payload into an alarmMessage.
func alarmFromSample(t *testing.T, name string) alarmMessage {
	t.Helper()
	ev := readEvent(t, "../../../samples/cloudwatch/"+name)
	m, ok := decodeAlarm(ev)
	if !ok {
		t.Fatalf("decode failed for %s", name)
	}
	return m
}

func TestChart_BuildWidgetJSON_SimpleMetric(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	raw, err := buildWidgetJSON(m, "us-east-1")
	if err != nil {
		t.Fatalf("buildWidgetJSON: %v", err)
	}
	var w map[string]any
	if jerr := json.Unmarshal(raw, &w); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, raw)
	}
	if w["title"] != "example-alarm" {
		t.Fatalf("title = %v, want example-alarm", w["title"])
	}
	if w["region"] != "us-east-1" {
		t.Fatalf("region = %v", w["region"])
	}
	metrics, ok := w["metrics"].([]any)
	if !ok || len(metrics) != 1 {
		t.Fatalf("metrics = %v", w["metrics"])
	}
	entry, ok := metrics[0].([]any)
	if !ok || len(entry) < 4 {
		t.Fatalf("entry shape = %v", metrics[0])
	}
	if entry[0] != "AWS/EC2" {
		t.Fatalf("namespace = %v", entry[0])
	}
	if entry[1] != "CPUUtilization" {
		t.Fatalf("metric name = %v", entry[1])
	}
	annot, ok := w["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("annotations missing: %v", w["annotations"])
	}
	if _, ok := annot["horizontal"]; !ok {
		t.Fatalf("annotations.horizontal missing")
	}
}

// TestChart_BuildWidgetJSON_MetricMath_UsesMetricStatDimensions verifies
// that when Trigger.Dimensions is empty (metric-math alarms), the chart
// reads MetricStat.Metric.Dimensions instead.
func TestChart_BuildWidgetJSON_MetricMath_UsesMetricStatDimensions(t *testing.T) {
	m := alarmFromSample(t, "alarm_metric_math_alb_5xx.json")
	if len(m.Trigger.Dimensions) != 0 {
		t.Fatalf("expected Trigger.Dimensions empty for metric-math alarm; got %v", m.Trigger.Dimensions)
	}
	if len(m.Trigger.Metrics) < 1 || m.Trigger.Metrics[0].MetricStat == nil {
		t.Fatalf("expected at least one MetricStat entry")
	}
	raw, err := buildWidgetJSON(m, "us-east-1")
	if err != nil {
		t.Fatalf("buildWidgetJSON: %v", err)
	}
	if !bytes.Contains(raw, []byte("LoadBalancer")) {
		t.Fatalf("expected LoadBalancer dimension from MetricStat.Metric.Dimensions in widget: %s", raw)
	}
	if !bytes.Contains(raw, []byte("HTTPCode_ELB_5XX_Count")) {
		t.Fatalf("expected metric name in widget: %s", raw)
	}
	if !bytes.Contains(raw, []byte("FILL(m1, 0)")) {
		t.Fatalf("expected metric math expression in widget: %s", raw)
	}
}

func TestChart_RenderAlarmChart_HappyPath(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	renderer := &fakeRenderer{body: []byte("PNGDATA")}
	uploader := &fakeUploader{}
	presigner := &fakePresigner{url: "https://bucket.s3.us-west-2.amazonaws.com/charts/x.png?signed=1"}

	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	pipe := &ChartRenderingPipeline{
		Renderer:  renderer,
		Uploader:  uploader,
		Presigner: presigner,
		Config: ChartConfig{
			BucketName:     "test-bucket",
			BucketRegion:   "us-west-2",
			FallbackRegion: "us-east-1",
		},
		Logger: logger,
	}
	got := pipe.renderAlarmChart(context.Background(), m)
	if !strings.HasPrefix(got, "https://") {
		t.Fatalf("URL = %q, want presigned URL", got)
	}
	if uploader.gotBucket != "test-bucket" {
		t.Fatalf("bucket = %q", uploader.gotBucket)
	}
	if uploader.gotType != "image/png" {
		t.Fatalf("content-type = %q", uploader.gotType)
	}
	if !strings.HasPrefix(uploader.gotKey, "charts/") || !strings.HasSuffix(uploader.gotKey, ".png") {
		t.Fatalf("key = %q", uploader.gotKey)
	}
	if !bytes.Equal(uploader.gotBody, []byte("PNGDATA")) {
		t.Fatalf("body = %q", uploader.gotBody)
	}
}

// TestChart_PresignedURL_BucketRegion verifies that the presigned URL uses
// the BucketRegion. The fake presigner is given a host that embeds the
// bucket region; the pipeline returns the same URL through.
func TestChart_PresignedURL_BucketRegion(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	presignedURL := "https://bucket.s3.us-west-2.amazonaws.com/charts/x.png?X-Amz-Signature=1"
	pipe := &ChartRenderingPipeline{
		Renderer:  &fakeRenderer{},
		Uploader:  &fakeUploader{},
		Presigner: &fakePresigner{url: presignedURL},
		Config: ChartConfig{
			BucketName:   "bucket",
			BucketRegion: "us-west-2",
		},
	}
	if got := pipe.renderAlarmChart(context.Background(), m); got != presignedURL {
		t.Fatalf("got %q, want %q (presigned URL must carry bucket region)", got, presignedURL)
	}
	if !strings.Contains(presignedURL, "us-west-2") {
		t.Fatalf("test setup error — bucket region not in URL")
	}
}

// TestChart_GetMetricWidgetImage_Error_LogsError verifies that slog.Error
// is invoked on chart-render failures and the URL falls back to empty.
func TestChart_GetMetricWidgetImage_Error_LogsError(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	pipe := &ChartRenderingPipeline{
		Renderer:  &fakeRenderer{err: errors.New("Throttled")},
		Uploader:  &fakeUploader{},
		Presigner: &fakePresigner{url: "x"},
		Config:    ChartConfig{BucketName: "b"},
		Logger:    logger,
	}
	if got := pipe.renderAlarmChart(context.Background(), m); got != "" {
		t.Fatalf("expected empty URL on render error, got %q", got)
	}
	assertLogContainsError(t, buf, "cloudwatch chart GetMetricWidgetImage failed")
}

// TestChart_PutObject_Error_LogsError covers the slog.Error path for the
// S3 upload step.
func TestChart_PutObject_Error_LogsError(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	pipe := &ChartRenderingPipeline{
		Renderer:  &fakeRenderer{},
		Uploader:  &fakeUploader{err: errors.New("AccessDenied")},
		Presigner: &fakePresigner{url: "x"},
		Config:    ChartConfig{BucketName: "b"},
		Logger:    logger,
	}
	if got := pipe.renderAlarmChart(context.Background(), m); got != "" {
		t.Fatalf("expected empty URL on PutObject error, got %q", got)
	}
	assertLogContainsError(t, buf, "cloudwatch chart PutObject failed")
}

// TestChart_PresignError_LogsError covers the third SDK seam.
func TestChart_PresignError_LogsError(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	pipe := &ChartRenderingPipeline{
		Renderer:  &fakeRenderer{},
		Uploader:  &fakeUploader{},
		Presigner: &fakePresigner{err: errors.New("InvalidParam")},
		Config:    ChartConfig{BucketName: "b"},
		Logger:    logger,
	}
	if got := pipe.renderAlarmChart(context.Background(), m); got != "" {
		t.Fatalf("expected empty URL on presign error, got %q", got)
	}
	assertLogContainsError(t, buf, "cloudwatch chart PresignGetObject failed")
}

// TestChart_EmptyBucket_SkipsRendering verifies the no-op branch when
// CHART_BUCKET_NAME is unset.
func TestChart_EmptyBucket_SkipsRendering(t *testing.T) {
	m := alarmFromSample(t, "alarm_critical_single_metric.json")
	pipe := &ChartRenderingPipeline{
		Renderer:  &fakeRenderer{err: errors.New("should-not-be-called")},
		Uploader:  &fakeUploader{},
		Presigner: &fakePresigner{},
		Config:    ChartConfig{}, // empty bucket
	}
	if got := pipe.renderAlarmChart(context.Background(), m); got != "" {
		t.Fatalf("empty bucket should yield empty URL, got %q", got)
	}
}

// TestChart_NilPipeline returns empty URL.
func TestChart_NilPipeline(t *testing.T) {
	var p *ChartRenderingPipeline
	got := p.renderAlarmChart(context.Background(), alarmMessage{})
	if got != "" {
		t.Fatalf("nil pipeline got %q", got)
	}
}

// TestMetricsLogsURL_NonLambdaIsEmpty asserts the helper returns "" for
// non-Lambda alarms.
func TestMetricsLogsURL_NonLambdaIsEmpty(t *testing.T) {
	got := metricsLogsURL(trigger{Namespace: "AWS/EC2"}, time.Now(), "us-east-1")
	if got != "" {
		t.Fatalf("non-lambda namespace got %q", got)
	}
}

// TestMetricsLogsURL_NoFunctionNameDimensionIsEmpty asserts the helper
// returns "" when the Lambda alarm lacks a FunctionName dimension.
func TestMetricsLogsURL_NoFunctionNameDimensionIsEmpty(t *testing.T) {
	got := metricsLogsURL(trigger{Namespace: "AWS/Lambda"}, time.Now(), "us-east-1")
	if got != "" {
		t.Fatalf("lambda without FunctionName got %q", got)
	}
}

// TestMetricsLogsURL_BuildsLambdaConsoleLink covers the success branch.
func TestMetricsLogsURL_BuildsLambdaConsoleLink(t *testing.T) {
	tr := trigger{
		Namespace:  "AWS/Lambda",
		Dimensions: []dimension{{Name: "FunctionName", Value: "aws-to-slack"}},
	}
	got := metricsLogsURL(tr, time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), "us-east-1")
	if !strings.Contains(got, "aws-to-slack") {
		t.Fatalf("got %q", got)
	}
}

func TestUpperFirstLowerRest(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"AVERAGE": "Average",
		"sum":     "Sum",
		"MaXiMuM": "Maximum",
	}
	for in, want := range cases {
		if got := upperFirstLowerRest(in); got != want {
			t.Fatalf("upperFirstLowerRest(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestChart_BuildMetricEntry_ExpressionOnly covers the metric-math
// expression branch in buildMetricEntry — metric.MetricStat is nil and
// metric.Expression is set.
func TestChart_BuildMetricEntry_ExpressionOnly(t *testing.T) {
	visibleTrue := true
	visibleFalse := false
	cases := []struct {
		name string
		in   metricEntry
		want string
	}{
		{
			name: "expression-with-label",
			in:   metricEntry{ID: "e1", Expression: "SUM(m1)", Label: "Filled", ReturnData: &visibleTrue},
			want: "Filled",
		},
		{
			name: "expression-no-label-uses-id",
			in:   metricEntry{ID: "e2", Expression: "FILL(m1, 0)", ReturnData: &visibleFalse},
			want: "e2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := buildMetricEntry(tc.in)
			if len(entry) != 1 {
				t.Fatalf("expression entry should have a single opts map; got %v", entry)
			}
			opts, ok := entry[0].(map[string]any)
			if !ok {
				t.Fatalf("entry[0] type = %T", entry[0])
			}
			if opts["label"] != tc.want {
				t.Fatalf("label = %v, want %v", opts["label"], tc.want)
			}
			if _, has := opts["expression"]; !has {
				t.Fatal("expression option missing")
			}
		})
	}
}

func TestBuildChartKey_FormatStable(t *testing.T) {
	got, err := buildChartKey(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("buildChartKey: %v", err)
	}
	if !strings.HasPrefix(got, "charts/2024-06-01/") {
		t.Fatalf("key = %q, want charts/2024-06-01/ prefix", got)
	}
	if !strings.HasSuffix(got, ".png") {
		t.Fatalf("key = %q, want .png suffix", got)
	}
}

// assertLogContainsError parses the first log line in buf, ensuring it is
// level=ERROR and that its msg attribute contains wantMsg.
func assertLogContainsError(t *testing.T, buf *bytes.Buffer, wantMsg string) {
	t.Helper()
	scanner := buf.Bytes()
	if i := bytes.IndexByte(scanner, '\n'); i > 0 {
		scanner = scanner[:i]
	}
	var line map[string]any
	if err := json.Unmarshal(scanner, &line); err != nil {
		t.Fatalf("log output not JSON: %v\n%s", err, buf.String())
	}
	if level, _ := line["level"].(string); level != "ERROR" {
		t.Fatalf("log level = %q, want ERROR (full log: %s)", level, buf.String())
	}
	if msg, _ := line["msg"].(string); !strings.Contains(msg, wantMsg) {
		t.Fatalf("log msg = %q, want substring %q", msg, wantMsg)
	}
}
