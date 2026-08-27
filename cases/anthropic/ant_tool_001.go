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
		caseutil.Assertion("A01", "Tool request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Tool response is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Requested tool is selected", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Tool use block has an ID", testcase.Must, testcase.High),
		caseutil.Assertion("A05", "Tool input contains the required value", testcase.Must, testcase.High),
	)
	definition := base("ANT-TOOL-001", "Anthropic forced tool use", "Forces a tool and validates its identity and structured input.", "tools", assertions, nil)
	definition.DependsOn = []string{"ANT-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		body := map[string]any{
			"model": target.Model, "max_tokens": 64,
			"messages":    []map[string]any{{"role": "user", "content": "Use get_weather for Hanoi. Do not answer directly."}},
			"tools":       []map[string]any{{"name": "get_weather", "description": "Get weather for a city.", "input_schema": map[string]any{"type": "object", "properties": map[string]any{"location": map[string]any{"type": "string"}}, "required": []string{"location"}}}},
			"tool_choice": map[string]any{"type": "tool", "name": "get_weather"},
		}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodPost, Path: "/messages", Body: body, Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "tool request did not succeed")
		}
		var payload struct {
			Content []struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input struct {
					Location string `json:"location"`
				} `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "tool response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		var call struct{ ID, Name, Location string }
		for _, content := range payload.Content {
			if content.Type == "tool_use" {
				call.ID, call.Name, call.Location = content.ID, content.Name, content.Input.Location
				break
			}
		}
		results = append(results, testcase.Result("A03", call.Name == "get_weather", "TOOL.NAME_MISMATCH", fmt.Sprintf("tool name = %q", call.Name)))
		results = append(results, testcase.Result("A04", call.ID != "", "TOOL.ID_EMPTY", "tool use ID is empty"))
		results = append(results, testcase.Result("A05", strings.TrimSpace(call.Location) != "", "TOOL.INPUT_INVALID", "tool input location is empty"))
		return testcase.Execution{Assertions: results}
	}
	register(definition)
}
