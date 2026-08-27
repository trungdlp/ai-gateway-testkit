package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Responses request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Responses body is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Response object discriminator is valid", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Response reaches completed state", testcase.Must, testcase.High),
		caseutil.Assertion("A05", "Response identifies a model", testcase.Must, testcase.Medium),
		caseutil.Assertion("A06", "Response contains non-empty output text", testcase.Must, testcase.High),
	)
	definition := base("OAI-RESP-001", "OpenAI non-streaming response", "Creates and validates a minimal non-streaming Responses API result.", "responses", assertions, nil)
	definition.DependsOn = []string{"OAI-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		body := map[string]any{"model": target.Model, "input": "Reply with a short greeting.", "max_output_tokens": 64, "store": false}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodPost, Path: "/responses", Body: body, Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "responses request did not succeed")
		}
		var payload struct {
			Object string `json:"object"`
			Status string `json:"status"`
			Model  string `json:"model"`
			Output []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "response body is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		results = append(results, testcase.Result("A03", payload.Object == "response", "SCHEMA.DISCRIMINATOR_MISMATCH", fmt.Sprintf("object = %q, want response", payload.Object)))
		results = append(results, testcase.Result("A04", payload.Status == "completed", "RESPONSE.NOT_COMPLETED", fmt.Sprintf("status = %q, want completed", payload.Status)))
		results = append(results, testcase.Result("A05", payload.Model != "", "SCHEMA.MODEL_EMPTY", "response model is empty"))
		var text string
		for _, output := range payload.Output {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					text += content.Text
				}
			}
		}
		results = append(results, testcase.Result("A06", strings.TrimSpace(text) != "", "RESPONSE.OUTPUT_EMPTY", "response output text is empty"))
		return testcase.Execution{Assertions: results, Evidence: map[string]any{"output_preview": caseutil.Truncate(environment.Redact(text), 160)}}
	}
	register(definition)
}
