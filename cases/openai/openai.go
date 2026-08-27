package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const protocol = "openai"

func Definitions() []testcase.Definition {
	return []testcase.Definition{auth(), models(), response(), toolCall(), sdk()}
}

func base(id, title, description, area string, assertions []testcase.AssertionDefinition, run testcase.RunFunc) testcase.Definition {
	return testcase.Definition{
		ID: id, Revision: 1, Title: title, Description: description, Layer: testcase.LayerProtocol,
		Suite: protocol, Area: area, Stability: testcase.StabilityStable, Determinism: testcase.Deterministic,
		SpecReferences: []testcase.SpecReference{{Title: "OpenAI API reference", URL: "https://developers.openai.com/api/reference/"}},
		Cost:           testcase.Cost{Requests: 1, MaxInputTokens: 128, MaxOutputTokens: 64},
		Risk:           testcase.Risk{Mutation: "none", ContainsOutput: area == "responses" || area == "tools"},
		Assertions:     assertions, Run: run,
	}
}

func auth() testcase.Definition {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Missing credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Invalid credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Credentials are not reflected in error responses", testcase.Must, testcase.Blocker),
	)
	definition := base("OAI-AUTH-001", "OpenAI authentication boundary", "Verifies rejection and non-reflection of missing and invalid bearer credentials.", "auth", assertions, nil)
	definition.Cost.Requests = 2
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		if _, ok := environment.Target(protocol); !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		missing, missingErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthNone})
		invalid, invalidErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthInvalid})
		results := make([]testcase.AssertionResult, 0, 3)
		if missingErr != nil {
			results = append(results, caseutil.RequestError("A01", missingErr))
		} else {
			ok := missing.StatusCode == http.StatusUnauthorized || missing.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A01", ok, "AUTH.MISSING_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", missing.StatusCode)))
		}
		if invalidErr != nil {
			results = append(results, caseutil.RequestError("A02", invalidErr))
		} else {
			ok := invalid.StatusCode == http.StatusUnauthorized || invalid.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A02", ok, "AUTH.INVALID_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", invalid.StatusCode)))
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
		caseutil.Assertion("A03", "Models response uses the list envelope", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Configured model is discoverable", testcase.Must, testcase.High),
	)
	definition := base("OAI-MODL-001", "OpenAI model discovery", "Validates the models list envelope and configured model discovery.", "models", assertions, nil)
	definition.DependsOn = []string{"OAI-AUTH-001"}
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
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "models response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		results = append(results, testcase.Result("A03", payload.Object == "list", "SCHEMA.ENVELOPE_MISMATCH", fmt.Sprintf("object = %q, want list", payload.Object)))
		found := false
		for _, model := range payload.Data {
			found = found || model.ID == target.Model
		}
		results = append(results, testcase.Result("A04", found, "MODEL.NOT_DISCOVERABLE", fmt.Sprintf("model %q was not returned", target.Model)))
		return testcase.Execution{Assertions: results}
	}
	return definition
}

func response() testcase.Definition {
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
	return definition
}

func toolCall() testcase.Definition {
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
				call.Type = candidate.Type
				call.Name = candidate.Name
				call.CallID = candidate.CallID
				call.Arguments = candidate.Arguments
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
	return definition
}

func sdk() testcase.Definition {
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
	return definition
}
