package behavioral

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const expected = "AGTK_OK"

func Definitions() []testcase.Definition {
	return []testcase.Definition{openAIInstruction(), anthropicInstruction()}
}

func behavioralBase(id, title, suite string, dependency string, run testcase.RunFunc) testcase.Definition {
	return testcase.Definition{
		ID: id, Revision: 1, Title: title,
		Description: "Checks exact instruction following independently from protocol schema compatibility.",
		Layer:       testcase.LayerBehavioral, Suite: suite, Area: "instruction_following",
		Stability: testcase.StabilityStable, Determinism: testcase.Probabilistic,
		DependsOn: []string{dependency}, Cost: testcase.Cost{Requests: 1, MaxInputTokens: 64, MaxOutputTokens: 16},
		Risk:       testcase.Risk{Mutation: "none", ContainsOutput: true},
		Assertions: caseutil.Assertions(caseutil.Assertion("A01", "Model follows an exact-output instruction", testcase.Should, testcase.Medium)),
		Run:        run,
	}
}

func openAIInstruction() testcase.Definition {
	return behavioralBase("BEH-OAI-001", "OpenAI exact instruction following", "openai", "OAI-RESP-001", func(ctx context.Context, environment testcase.Environment) testcase.Execution {
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
	})
}

func anthropicInstruction() testcase.Definition {
	return behavioralBase("BEH-ANT-001", "Anthropic exact instruction following", "anthropic", "ANT-MSG-001", func(ctx context.Context, environment testcase.Environment) testcase.Execution {
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
	})
}

func exactResult(environment testcase.Environment, output string) testcase.Execution {
	trimmed := strings.TrimSpace(output)
	message := fmt.Sprintf("output = %q, want %q", caseutil.Truncate(environment.Redact(trimmed), 160), expected)
	return testcase.Execution{
		Assertions: []testcase.AssertionResult{testcase.Result("A01", trimmed == expected, "BEHAVIOR.EXACT_OUTPUT_MISMATCH", message)},
		Evidence:   map[string]any{"output_preview": caseutil.Truncate(environment.Redact(trimmed), 160)},
	}
}
