package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/cases"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/target"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestEngineRunsOpenAICoreProfile(t *testing.T) {
	t.Parallel()
	server := openAIServer(t, "test-model")
	defer server.Close()
	catalog, err := cases.Catalog()
	require.NoError(t, err)
	profiles, err := profile.Load(catalog)
	require.NoError(t, err)
	targets := target.Resolved{Name: "fixture", Endpoints: map[string]testcase.Target{"openai": {Protocol: "openai", BaseURL: server.URL + "/v1", Model: "test-model", Credential: "test-secret", Timeout: time.Second, Retry: gateway.RetryPolicy{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}}}}

	commit := "0123456789012345678901234567890123456789"
	reportValue, err := New(catalog, profiles, targets).Run(context.Background(), Options{Profiles: []string{"oai-tools", "oai-sdk-go"}, Build: result.Build{Commit: commit}})

	require.NoError(t, err)
	require.Len(t, reportValue.Profiles, 2)
	assert.Equal(t, result.VerdictPass, reportValue.Profiles[0].Verdict)
	assert.Equal(t, result.VerdictPass, reportValue.Profiles[1].Verdict)
	assert.Equal(t, reportValue.Summary.Total, reportValue.Summary.Passed)
	assert.NotEmpty(t, reportValue.CatalogDigest)
	assert.Contains(t, reportValue.SchemaURL, commit)
}

func TestEngineRecordsRecoveredTransientRetry(t *testing.T) {
	t.Parallel()
	responseAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/models":
			_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"test-model"}]}`))
		case "/v1/responses":
			responseAttempts++
			if responseAttempts == 1 {
				writer.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = writer.Write([]byte(`{"object":"response","status":"completed","model":"test-model","output":[{"content":[{"type":"output_text","text":"hello"}]}]}`))
		}
	}))
	defer server.Close()
	catalog, err := cases.Catalog()
	require.NoError(t, err)
	profiles, err := profile.Load(catalog)
	require.NoError(t, err)
	retry := gateway.RetryPolicy{MaxRetries: 2, InitialBackoff: time.Nanosecond, MaxBackoff: time.Nanosecond}
	targets := target.Resolved{Name: "fixture", Endpoints: map[string]testcase.Target{"openai": {Protocol: "openai", BaseURL: server.URL + "/v1", Model: "test-model", Credential: "test-secret", Timeout: time.Second, Retry: retry}}}

	reportValue, err := New(catalog, profiles, targets).Run(context.Background(), Options{Profiles: []string{"oai-core"}})

	require.NoError(t, err)
	assert.Equal(t, result.VerdictPass, reportValue.Profiles[0].Verdict)
	assert.Equal(t, int64(1), reportValue.Runtime.Retry.RawHTTPRetries)
	assert.Equal(t, int64(5), reportValue.Runtime.Retry.RawHTTPAttempts)
	for _, scenario := range reportValue.Scenarios {
		if scenario.ID == "OAI-RESP-001" {
			assert.Equal(t, map[string]int64{"retries": 1, "exhausted": 0}, scenario.Evidence["retry"])
		}
	}
}

func TestEngineRunsAnthropicToolAndSDKProfiles(t *testing.T) {
	t.Parallel()
	server := anthropicServer(t, "test-model")
	defer server.Close()
	catalog, err := cases.Catalog()
	require.NoError(t, err)
	profiles, err := profile.Load(catalog)
	require.NoError(t, err)
	targets := target.Resolved{Name: "fixture", Endpoints: map[string]testcase.Target{"anthropic": {Protocol: "anthropic", BaseURL: server.URL + "/v1", Model: "test-model", Credential: "test-secret", Timeout: time.Second, APIVersion: "2023-06-01"}}}

	reportValue, err := New(catalog, profiles, targets).Run(context.Background(), Options{Profiles: []string{"anthropic-tools", "anthropic-sdk-go"}})

	require.NoError(t, err)
	require.Len(t, reportValue.Profiles, 2)
	assert.Equal(t, result.VerdictPass, reportValue.Profiles[0].Verdict)
	assert.Equal(t, result.VerdictPass, reportValue.Profiles[1].Verdict)
}

func openAIServer(t *testing.T, model string) *httptest.Server {
	t.Helper()
	plainResponses := 0
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		credential := request.Header.Get("Authorization")
		if credential != "Bearer test-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":{"message":"unauthorized"}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/models":
			fmt.Fprintf(writer, `{"object":"list","data":[{"id":%q}]}`, model)
		case "/v1/responses":
			var body struct {
				Tools []json.RawMessage `json:"tools"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			if len(body.Tools) > 0 {
				fmt.Fprintf(writer, `{"object":"response","status":"completed","model":%q,"output":[{"type":"function_call","name":"get_weather","call_id":"call_1","arguments":"{\"location\":\"Hanoi\"}"}]}`, model)
				return
			}
			plainResponses++
			if plainResponses == 2 {
				writer.WriteHeader(http.StatusBadGateway)
				return
			}
			fmt.Fprintf(writer, `{"id":"resp_1","object":"response","status":"completed","model":%q,"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}]}`, model)
		default:
			http.NotFound(writer, request)
		}
	}))
}

func anthropicServer(t *testing.T, model string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-API-Key") != "test-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"unauthorized"}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/models":
			fmt.Fprintf(writer, `{"data":[{"id":%q,"type":"model"}],"has_more":false,"first_id":%q,"last_id":%q}`, model, model, model)
		case "/v1/messages":
			var body struct {
				Tools []json.RawMessage `json:"tools"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			if len(body.Tools) > 0 {
				fmt.Fprintf(writer, `{"id":"msg_1","type":"message","role":"assistant","model":%q,"content":[{"type":"tool_use","id":"tool_1","name":"get_weather","input":{"location":"Hanoi"}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":10}}`, model)
				return
			}
			fmt.Fprintf(writer, `{"id":"msg_1","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":10}}`, model)
		default:
			http.NotFound(writer, request)
		}
	}))
}
