package openai

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	openaisdk "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Official SDK request and deserialization succeed", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Official SDK exposes non-empty output text", testcase.Must, testcase.High),
	)
	definition := base("OAI-SDK-001", "OpenAI Go SDK interoperability", "Uses the official OpenAI Go SDK against the configured gateway.", "sdk", assertions, nil)
	definition.Layer = testcase.LayerSDK
	definition.DependsOn = []string{"OAI-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		client := openaisdk.NewClient(openaioption.WithAPIKey(target.Credential), openaioption.WithBaseURL(target.BaseURL+"/"), openaioption.WithHTTPClient(&http.Client{Timeout: target.Timeout}), openaioption.WithMaxRetries(target.Retry.MaxRetries))
		response, err := client.Responses.New(ctx, responses.ResponseNewParams{Model: shared.ResponsesModel(target.Model), Input: responses.ResponseNewParamsInputUnion{OfString: openaisdk.String("Reply with a short greeting.")}, MaxOutputTokens: openaisdk.Int(64), Store: openaisdk.Bool(false)})
		if err != nil {
			message := environment.Redact(err.Error())
			var apiError *openaisdk.Error
			var networkError net.Error
			if (errors.As(err, &apiError) && gateway.IsTransientStatus(apiError.StatusCode)) || errors.As(err, &networkError) || errors.Is(err, context.Canceled) {
				return caseutil.ErrorAll(assertions, "SDK.TRANSIENT_EXHAUSTED", message)
			}
			return caseutil.FailAll(assertions, "SDK.INCOMPATIBLE", message)
		}
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.Pass("A01"), testcase.Result("A02", strings.TrimSpace(response.OutputText()) != "", "SDK.OUTPUT_EMPTY", "SDK parsed an empty output")}}
	}
	register(definition)
}
