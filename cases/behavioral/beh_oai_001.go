package behavioral

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	register(behavioralBase("BEH-OAI-001", "OpenAI exact instruction following", "openai", "OAI-RESP-001", runBEHOAI001))
}

func runBEHOAI001(ctx context.Context, environment testcase.Environment) testcase.Execution {
	target, ok := environment.Target("openai")
	if !ok {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.NotApplicable("A01", "openai target is not configured")}}
	}
	body := map[string]any{"model": target.Model, "input": "Reply with exactly AGTK_OK and nothing else.", "max_output_tokens": 16, "store": false}
	response, err := environment.DoJSON(ctx, testcase.Request{Protocol: "openai", Method: http.MethodPost, Path: "/responses", Body: body, Auth: testcase.AuthValid})
	if err != nil {
		return testcase.Execution{Assertions: []testcase.AssertionResult{caseutil.RequestError("A01", err)}}
	}
	if response.StatusCode != http.StatusOK {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.Fail("A01", "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}}
	}
	var payload struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return testcase.Execution{Assertions: []testcase.AssertionResult{testcase.Fail("A01", "JSON.INVALID", err.Error())}}
	}
	var output string
	for _, item := range payload.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" {
				output += content.Text
			}
		}
	}
	return exactResult(environment, output)
}
