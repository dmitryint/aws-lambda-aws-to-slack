package router_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/envelope"
	"github.com/esai-dev/aws-lambda-aws-to-slack/internal/parser"
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
)

// registeredParsers mirrors the production parser order through Wave 6.
// The generic parser is always last and is expected to also claim every
// event (it is the universal catch-all). The contract this test enforces
// is that exactly one non-generic parser matches each fixture.
func registeredParsers() []parser.Parser {
	return []parser.Parser{
		autoscalingparser.New(),
		awshealthparser.New(),
		batchparser.New(),
		beanstalkparser.New(),
		cloudformationparser.New(),
		cloudwatchparser.New(),
		codebuildparser.New(),
		codecommitparser.NewPullRequest(),
		codecommitparser.NewRepository(),
		codedeployparser.NewCloudWatch(),
		codedeployparser.NewSNS(),
		codepipelineparser.New(),
		codepipelineparser.NewApproval(),
		guarddutyparser.New(),
		inspectorparser.New(),
		inspector2parser.New(),
		rdsparser.New(),
		ecsparser.New(),
		sesparser.NewBounce(),
		sesparser.NewComplaint(),
		sesparser.NewReceived(),
		genericparser.New(),
	}
}

// sampleDirs lists the per-source sample directories Waves 2-6 own. Each
// fixture in these directories must match exactly one non-generic parser.
var sampleDirs = map[string]string{
	"autoscaling":            "../../samples/autoscaling",
	"awshealth":              "../../samples/awshealth",
	"batch":                  "../../samples/batch",
	"beanstalk":              "../../samples/beanstalk",
	"cloudformation":         "../../samples/cloudformation",
	"cloudwatch":             "../../samples/cloudwatch",
	"codebuild":              "../../samples/codebuild",
	"codecommit-pullrequest": "../../samples/codecommit/pullrequest",
	"codecommit-repository":  "../../samples/codecommit/repository",
	"codedeploy-cloudwatch":  "../../samples/codedeploy/eventbridge",
	"codedeploy-sns":         "../../samples/codedeploy/sns",
	"codepipeline":           "../../samples/codepipeline",
	"codepipeline-approval":  "../../samples/codepipeline/approval",
	"guardduty":              "../../samples/guardduty",
	"inspector":              "../../samples/inspector/classic",
	"inspector2":             "../../samples/inspector2",
	"rds":                    "../../samples/rds",
	"ecs":                    "../../samples/ecs",
	"ses-bounce":             "../../samples/ses/bounce",
	"ses-complaint":          "../../samples/ses/complaint",
	"ses-received":           "../../samples/ses/received",
}

// expectedOwner maps each sample subdirectory key to the parser name that
// must claim its fixtures (in addition to the generic catch-all).
var expectedOwner = map[string]string{
	"autoscaling":            "autoscaling",
	"awshealth":              "awshealth",
	"batch":                  "batch",
	"beanstalk":              "beanstalk",
	"cloudformation":         "cloudformation",
	"cloudwatch":             "cloudwatch",
	"codebuild":              "codebuild",
	"codecommit-pullrequest": "codecommit-pullrequest",
	"codecommit-repository":  "codecommit-repository",
	"codedeploy-cloudwatch":  "codedeploy-cloudwatch",
	"codedeploy-sns":         "codedeploy-sns",
	"codepipeline":           "codepipeline",
	"codepipeline-approval":  "codepipeline-approval",
	"guardduty":              "guardduty",
	"inspector":              "inspector",
	"inspector2":             "inspector2",
	"rds":                    "rds",
	"ecs":                    "ecs",
	"ses-bounce":             "ses-bounce",
	"ses-complaint":          "ses-complaint",
	"ses-received":           "ses-received",
}

func TestRouter_WaveSixCoverage(t *testing.T) {
	parsers := registeredParsers()
	r := router.New()
	for _, p := range parsers {
		r.Register(p)
	}
	registered := r.Parsers()
	if got := registered[len(registered)-1].Name(); got != "generic" {
		t.Fatalf("generic must be last; tail = %q", got)
	}

	for source, dir := range sampleDirs {
		t.Run(source, func(t *testing.T) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read samples %s: %v", dir, err)
			}
			for _, entry := range entries {
				name := entry.Name()
				if !strings.HasSuffix(name, ".json") {
					continue
				}
				t.Run(name, func(t *testing.T) {
					raw, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // test fixture path
					if err != nil {
						t.Fatalf("read sample: %v", err)
					}
					ev, err := envelope.New(json.RawMessage(raw))
					if err != nil {
						t.Fatalf("envelope.New: %v", err)
					}
					rec := ev.Records()[0]
					var nonGenericMatches []string
					for _, p := range registered {
						if p.Name() == "generic" {
							continue
						}
						if p.Match(rec) {
							nonGenericMatches = append(nonGenericMatches, p.Name())
						}
					}
					if len(nonGenericMatches) != 1 {
						t.Fatalf("expected exactly 1 non-generic parser match, got %d: %v",
							len(nonGenericMatches), nonGenericMatches)
					}
					if got, want := nonGenericMatches[0], expectedOwner[source]; got != want {
						t.Fatalf("owner = %q, want %q for %s", got, want, name)
					}
				})
			}
		})
	}
}

// TestRouter_RegistrationOrder asserts the documented parser order. New waves
// extend this list as they implement more parsers.
func TestRouter_RegistrationOrder(t *testing.T) {
	parsers := registeredParsers()
	want := []string{
		"autoscaling",
		"awshealth",
		"batch",
		"beanstalk",
		"cloudformation",
		"cloudwatch",
		"codebuild",
		"codecommit-pullrequest",
		"codecommit-repository",
		"codedeploy-cloudwatch",
		"codedeploy-sns",
		"codepipeline",
		"codepipeline-approval",
		"guardduty",
		"inspector",
		"inspector2",
		"rds",
		"ecs",
		"ses-bounce",
		"ses-complaint",
		"ses-received",
		"generic",
	}
	if len(parsers) != len(want) {
		t.Fatalf("parser count = %d, want %d", len(parsers), len(want))
	}
	got := make([]string, len(parsers))
	for i, p := range parsers {
		got[i] = p.Name()
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("parser[%d] = %q, want %q (full = %s)", i, got[i], w, fmt.Sprintf("%v", got))
		}
	}
}
