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
		caseutil.Assertion("A01", "Tool request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Tool response is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Requested function is selected", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Function call has a call ID", testcase.Must, testcase.High),
		caseutil.Assertion("A05", "Function arguments are valid JSON", testcase.Must, testcase.High),
	)
	definition := base("OAI-TOOL-001", "OpenAI forced function call", "Forces a function tool and validates its call identity and arguments.", "tools", assertions, nil)
	definition.DependsOn = []string{"OAI-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		body := map[string]any{"model": target.Model, "input": "Use get_weather for Hanoi. Do not answer directly.", "tools": []map[string]any{{"type": "function", "name": "get_weather", "description": "Get weather for a city.", "strict": true, "parameters": map[string]any{"type": "object", "properties": map[string]any{"location": map[string]any{"type": "string"}}, "required": []string{"location"}, "additionalProperties": false}}}, "tool_choice": map[string]any{"type": "function", "name": "get_weather"}, "max_output_tokens": 64, "store": false}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodPost, Path: "/responses", Body: body, Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "tool request did not succeed")
		}
		var payload struct {
			Output []struct {
				Type      string `json:"type"`
				Name      string `json:"name"`
				CallID    string `json:"call_id"`
				Arguments string `json:"arguments"`
			} `json:"output"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "tool response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		var call struct{ Type, Name, CallID, Arguments string }
		for _, candidate := range payload.Output {
			if candidate.Type == "function_call" {
				call.Type, call.Name, call.CallID, call.Arguments = candidate.Type, candidate.Name, candidate.CallID, candidate.Arguments
				break
			}
		}
		results = append(results, testcase.Result("A03", call.Name == "get_weather", "TOOL.NAME_MISMATCH", fmt.Sprintf("tool name = %q", call.Name)))
		results = append(results, testcase.Result("A04", call.CallID != "", "TOOL.CALL_ID_EMPTY", "function call_id is empty"))
		var arguments struct {
			Location string `json:"location"`
		}
		argumentsErr := json.Unmarshal([]byte(call.Arguments), &arguments)
		results = append(results, testcase.Result("A05", argumentsErr == nil && strings.TrimSpace(arguments.Location) != "", "TOOL.ARGUMENTS_INVALID_JSON", "function arguments do not contain a valid location"))
		return testcase.Execution{Assertions: results}
	}
	register(definition)
}
