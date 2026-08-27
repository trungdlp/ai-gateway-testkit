package behavioral

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	register(behavioralBase("BEH-ANT-001", "Anthropic exact instruction following", "anthropic", "ANT-MSG-001", runBEHANT001))
}

func runBEHANT001(ctx context.Context, environment testcase.Environment) testcase.Execution {
	target, ok := environment.Target("anthropic")
	if !ok {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.NotApplicable("A01", "anthropic target is not configured")}}
	}
	body := map[string]any{"model": target.Model, "max_tokens": 16, "messages": []map[string]any{{"role": "user", "content": "Reply with exactly AGTK_OK and nothing else."}}}
	response, err := environment.DoJSON(ctx, testcase.Request{Protocol: "anthropic", Method: http.MethodPost, Path: "/messages", Body: body, Auth: testcase.AuthValid})
	if err != nil {
		return testcase.Execution{Assertions: []testcase.AssertionResult{caseutil.RequestError("A01", err)}}
	}
	if response.StatusCode != http.StatusOK {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.Fail("A01", "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}}
	}
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.Fail("A01", "JSON.INVALID", err.Error())}}
	}
	var output string
	for _, content := range payload.Content {
		if content.Type == "text" {
			output += content.Text
		}
	}
	return exactResult(environment, output)
}
