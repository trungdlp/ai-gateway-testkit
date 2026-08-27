package anthropic

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Official SDK request and deserialization succeed", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Official SDK exposes non-empty output text", testcase.Must, testcase.High),
	)
	definition := base("ANT-SDK-001", "Anthropic Go SDK interoperability", "Uses the official Anthropic Go SDK against the configured gateway.", "sdk", assertions, nil)
	definition.Layer = testcase.LayerSDK
	definition.DependsOn = []string{"ANT-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		client := anthropicsdk.NewClient(
			anthropicoption.WithAPIKey(target.Credential),
			anthropicoption.WithBaseURL(strings.TrimSuffix(target.BaseURL, "/v1")+"/"),
			anthropicoption.WithHTTPClient(&http.Client{Timeout: target.Timeout}),
			anthropicoption.WithMaxRetries(target.Retry.MaxRetries),
		)
		message, err := client.Messages.New(ctx, anthropicsdk.MessageNewParams{
			Model: anthropicsdk.Model(target.Model), MaxTokens: 64,
			Messages: []anthropicsdk.MessageParam{anthropicsdk.NewUserMessage(anthropicsdk.NewTextBlock("Reply with a short greeting."))},
		})
		if err != nil {
			message := environment.Redact(err.Error())
			var apiError *anthropicsdk.Error
			var networkError net.Error
			if (errors.As(err, &apiError) && gateway.IsTransientStatus(apiError.StatusCode)) || errors.As(err, &networkError) || errors.Is(err, context.Canceled) {
				return caseutil.ErrorAll(assertions, "SDK.TRANSIENT_EXHAUSTED", message)
			}
			return caseutil.FailAll(assertions, "SDK.INCOMPATIBLE", message)
		}
		var output string
		for _, content := range message.Content {
			if content.Type == "text" {
				output += content.Text
			}
		}
		return testcase.Execution{Assertions: []testcase.AssertionResult{
			testcase.Pass("A01"),
			testcase.Result("A02", strings.TrimSpace(output) != "", "SDK.OUTPUT_EMPTY", "SDK parsed an empty output"),
		}}
	}
	register(definition)
}
