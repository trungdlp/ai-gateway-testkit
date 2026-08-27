package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const protocol = "anthropic"

func Definitions() []testcase.Definition {
	return []testcase.Definition{auth(), models(), message(), toolCall(), sdk()}
}

func base(id, title, description, area string, assertions []testcase.AssertionDefinition, run testcase.RunFunc) testcase.Definition {
	return testcase.Definition{
		ID: id, Revision: 1, Title: title, Description: description, Layer: testcase.LayerProtocol,
		Suite: protocol, Area: area, Stability: testcase.StabilityStable, Determinism: testcase.Deterministic,
		SpecReferences: []testcase.SpecReference{{Title: "Anthropic API reference", URL: "https://docs.anthropic.com/en/api/overview"}},
		Cost:           testcase.Cost{Requests: 1, MaxInputTokens: 128, MaxOutputTokens: 64},
		Risk:           testcase.Risk{Mutation: "none", ContainsOutput: area == "messages" || area == "tools"},
		Assertions:     assertions, Run: run,
	}
}

func auth() testcase.Definition {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Missing credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Invalid credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Credentials are not reflected in error responses", testcase.Must, testcase.Blocker),
	)
	definition := base("ANT-AUTH-001", "Anthropic authentication boundary", "Verifies rejection and non-reflection of missing and invalid API keys.", "auth", assertions, nil)
	definition.Cost.Requests = 2
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		if _, ok := environment.Target(protocol); !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		missing, missingErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthNone})
		invalid, invalidErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthInvalid})
		results := make([]testcase.AssertionResult, 0, len(assertions))
		if missingErr != nil {
			results = append(results, caseutil.RequestError("A01", missingErr))
		} else {
			valid := missing.StatusCode == http.StatusUnauthorized || missing.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A01", valid, "AUTH.MISSING_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", missing.StatusCode)))
		}
		if invalidErr != nil {
			results = append(results, caseutil.RequestError("A02", invalidErr))
		} else {
			valid := invalid.StatusCode == http.StatusUnauthorized || invalid.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A02", valid, "AUTH.INVALID_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", invalid.StatusCode)))
		}
		if missingErr != nil || invalidErr != nil {
			results = append(results, testcase.Error("A03", "AUTH.EVIDENCE_UNAVAILABLE", "authentication response evidence is incomplete"))
		} else {
			reflected := strings.Contains(string(missing.Body), "agtk-invalid-credential") || strings.Contains(string(invalid.Body), "agtk-invalid-credential")
			results = append(results, testcase.Result("A03", !reflected, "AUTH.CREDENTIAL_REFLECTED", "an error response reflected a supplied credential"))
		}
		return testcase.Execution{Assertions: results}
	}
	return definition
}

func models() testcase.Definition {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Models request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Models response is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Model entries use the model discriminator", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Configured model is discoverable", testcase.Must, testcase.High),
	)
	definition := base("ANT-MODL-001", "Anthropic model discovery", "Validates model entry discriminators and configured model discovery.", "models", assertions, nil)
	definition.DependsOn = []string{"ANT-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "models request did not succeed")
		}
		var payload struct {
			Data []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "models response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		validTypes := len(payload.Data) > 0
		found := false
		for _, model := range payload.Data {
			validTypes = validTypes && model.Type == "model"
			found = found || model.ID == target.Model
		}
		results = append(results, testcase.Result("A03", validTypes, "SCHEMA.MODEL_TYPE_MISMATCH", "one or more model entries do not have type model"))
		results = append(results, testcase.Result("A04", found, "MODEL.NOT_DISCOVERABLE", fmt.Sprintf("model %q was not returned", target.Model)))
		return testcase.Execution{Assertions: results}
	}
	return definition
}

func message() testcase.Definition {
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
	return definition
}

func toolCall() testcase.Definition {
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
	return definition
}

func sdk() testcase.Definition {
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
	return definition
}
