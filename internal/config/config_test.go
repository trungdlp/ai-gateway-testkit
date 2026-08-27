package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseURL   string
		model     string
		apiKey    string
		timeout   time.Duration
		allowHTTP bool
		wantURL   string
		wantErr   string
	}{
		{name: "https", baseURL: "https://gateway.example/v1/", model: "opus", apiKey: "secret", timeout: time.Second, wantURL: "https://gateway.example/v1"},
		{name: "loopback HTTP", baseURL: "http://127.0.0.1:8080/v1", model: "opus", apiKey: "secret", timeout: time.Second, wantURL: "http://127.0.0.1:8080/v1"},
		{name: "explicit insecure HTTP", baseURL: "http://gateway.example/v1", model: "opus", apiKey: "secret", timeout: time.Second, allowHTTP: true, wantURL: "http://gateway.example/v1"},
		{name: "remote HTTP rejected", baseURL: "http://gateway.example/v1", model: "opus", apiKey: "secret", timeout: time.Second, wantErr: "plain HTTP"},
		{name: "missing model", baseURL: "https://gateway.example/v1", apiKey: "secret", timeout: time.Second, wantErr: "model is required"},
		{name: "missing key", baseURL: "https://gateway.example/v1", model: "opus", timeout: time.Second, wantErr: "API key is required"},
		{name: "query rejected", baseURL: "https://gateway.example/v1?debug=true", model: "opus", apiKey: "secret", timeout: time.Second, wantErr: "query or fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := New(tt.baseURL, tt.model, tt.apiKey, tt.timeout, ProtocolOpenAI, tt.allowHTTP)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, got.BaseURL)
		})
	}
}

func TestParseProtocol(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"openai", "anthropic", "both"} {
		got, err := ParseProtocol(value)
		require.NoError(t, err)
		assert.Equal(t, value, string(got))
	}
	_, err := ParseProtocol("unknown")
	assert.Error(t, err)
}
