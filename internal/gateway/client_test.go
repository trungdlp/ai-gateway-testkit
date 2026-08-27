package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientDoJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/responses", r.URL.Path)
		assert.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"response"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", time.Second)
	headers := http.Header{"Authorization": []string{"Bearer test-secret"}}
	response, err := client.DoJSON(context.Background(), http.MethodPost, "/responses", map[string]any{"model": "test"}, headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, response.StatusCode)
}

func TestClientRetriesTransientStatusAndReplaysBody(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "test", body["model"])
		if requests == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClientWithRetry(server.URL, time.Second, RetryPolicy{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond})
	client.sleep = func(context.Context, time.Duration) error { return nil }

	response, err := client.DoJSON(context.Background(), http.MethodPost, "/responses", map[string]any{"model": "test"}, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 2, response.Attempts)
	assert.Equal(t, Stats{Requests: 1, Attempts: 2, Retries: 1}, client.Stats())
}

func TestClientReturnsTransientErrorAfterRetries(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewClientWithRetry(server.URL, time.Second, RetryPolicy{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	var delays []time.Duration
	client.sleep = func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }

	response, err := client.DoJSON(context.Background(), http.MethodGet, "/models", nil, nil)

	var transient *TransientError
	require.True(t, errors.As(err, &transient))
	assert.Equal(t, 3, response.Attempts)
	assert.Equal(t, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}, delays)
	assert.Equal(t, int64(1), client.Stats().Exhausted)
}

func TestClientDoesNotRetryStableClientError(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client := NewClientWithRetry(server.URL, time.Second, RetryPolicy{MaxRetries: 3})

	response, err := client.DoJSON(context.Background(), http.MethodGet, "/models", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	assert.Equal(t, 1, requests)
}

func TestClientOmitsAuthorization(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	_, err := client.DoJSON(context.Background(), http.MethodGet, "/models", nil, nil)
	assert.NoError(t, err)
}
