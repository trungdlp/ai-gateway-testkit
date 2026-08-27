package anthropic

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
		caseutil.Assertion("A01", "Messages request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Messages response is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Message discriminator and role are valid", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Message identifies a model", testcase.Must, testcase.Medium),
		caseutil.Assertion("A05", "Message contains non-empty text", testcase.Must, testcase.High),
	)
	definition := base("ANT-MSG-001", "Anthropic non-streaming message", "Creates and validates a minimal non-streaming Messages API result.", "messages", assertions, nil)
	definition.DependsOn = []string{"ANT-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		body := map[string]any{"model": target.Model, "max_tokens": 64, "messages": []map[string]any{{"role": "user", "content": "Reply with a short greeting."}}}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodPost, Path: "/messages", Body: body, Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "messages request did not succeed")
		}
		var payload struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Model   string `json:"model"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "message response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		results = append(results, testcase.Result("A03", payload.Type == "message" && payload.Role == "assistant", "SCHEMA.MESSAGE_METADATA_MISMATCH", fmt.Sprintf("type = %q, role = %q", payload.Type, payload.Role)))
		results = append(results, testcase.Result("A04", payload.Model != "", "SCHEMA.MODEL_EMPTY", "message model is empty"))
		var output string
		for _, content := range payload.Content {
			if content.Type == "text" {
				output += content.Text
			}
		}
		results = append(results, testcase.Result("A05", strings.TrimSpace(output) != "", "MESSAGE.OUTPUT_EMPTY", "message text is empty"))
		return testcase.Execution{Assertions: results, Evidence: map[string]any{"output_preview": caseutil.Truncate(environment.Redact(output), 160)}}
	}
	register(definition)
}
