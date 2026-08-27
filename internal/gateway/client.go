package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const maxResponseBody = 4 << 20

type RetryPolicy struct {
	MaxRetries     int           `json:"max_retries"`
	InitialBackoff time.Duration `json:"initial_backoff"`
	MaxBackoff     time.Duration `json:"max_backoff"`
}

type Stats struct {
	Requests  int64 `json:"requests"`
	Attempts  int64 `json:"attempts"`
	Retries   int64 `json:"retries"`
	Exhausted int64 `json:"exhausted"`
}

type TransientError struct {
	StatusCode int
	Attempts   int
	Cause      error
}

func (e *TransientError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("transient HTTP status %d persisted after %d attempts", e.StatusCode, e.Attempts)
	}
	return fmt.Sprintf("transient transport error persisted after %d attempts: %v", e.Attempts, e.Cause)
}

func (e *TransientError) Unwrap() error { return e.Cause }

// Client performs bounded HTTP requests against an AI gateway.
type Client struct {
	baseURL    string
	httpClient *http.Client
	retry      RetryPolicy
	sleep      func(context.Context, time.Duration) error
	requests   atomic.Int64
	attempts   atomic.Int64
	retries    atomic.Int64
	exhausted  atomic.Int64
}

// Response is the transport-level result returned to a conformance check.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Duration   time.Duration
	Attempts   int
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return NewClientWithRetry(baseURL, timeout, RetryPolicy{})
}

func NewClientWithRetry(baseURL string, timeout time.Duration, retry RetryPolicy) *Client {
	if retry.InitialBackoff <= 0 {
		retry.InitialBackoff = 250 * time.Millisecond
	}
	if retry.MaxBackoff <= 0 {
		retry.MaxBackoff = 5 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		retry:      retry,
		sleep:      sleepContext,
	}
}

// DoJSON sends a JSON request. Stable non-2xx responses are returned so cases
// can assert them. Exhausted transient responses return a TransientError.
func (c *Client) DoJSON(ctx context.Context, method, path string, body any, headers http.Header) (Response, error) {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return Response{}, fmt.Errorf("encode request body: %w", err)
		}
	}
	c.requests.Add(1)
	started := time.Now()
	for attempt := 1; ; attempt++ {
		c.attempts.Add(1)
		response, err := c.attempt(ctx, method, path, encoded, body != nil, headers)
		response.Attempts = attempt
		response.Duration = time.Since(started)
		retryable := IsTransientStatus(response.StatusCode) || isTransientError(err)
		if err == nil && !retryable {
			return response, nil
		}
		if err != nil && !retryable {
			return response, err
		}
		if attempt > c.retry.MaxRetries || ctx.Err() != nil {
			c.exhausted.Add(1)
			return response, &TransientError{StatusCode: response.StatusCode, Attempts: attempt, Cause: err}
		}
		c.retries.Add(1)
		if err := c.sleep(ctx, retryDelay(c.retry, attempt, response.Header)); err != nil {
			c.exhausted.Add(1)
			return response, &TransientError{StatusCode: response.StatusCode, Attempts: attempt, Cause: err}
		}
	}
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var networkError net.Error
	return errors.As(err, &networkError) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Client) attempt(ctx context.Context, method, path string, encoded []byte, hasBody bool, headers http.Header) (Response, error) {
	var requestBody io.Reader
	if hasBody {
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/"+strings.TrimLeft(path, "/"), requestBody)
	if err != nil {
		return Response{}, fmt.Errorf("create %s %s request: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("perform %s %s request: %w", method, path, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	closeErr := resp.Body.Close()
	response := Response{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: data}
	if readErr != nil {
		return response, fmt.Errorf("read %s %s response: %w", method, path, readErr)
	}
	if closeErr != nil {
		return response, fmt.Errorf("close %s %s response: %w", method, path, closeErr)
	}
	if len(data) > maxResponseBody {
		return response, fmt.Errorf("%s %s response exceeds %d bytes", method, path, maxResponseBody)
	}
	return response, nil
}

func (c *Client) Stats() Stats {
	return Stats{Requests: c.requests.Load(), Attempts: c.attempts.Load(), Retries: c.retries.Load(), Exhausted: c.exhausted.Load()}
}

func IsTransientStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryDelay(policy RetryPolicy, attempt int, header http.Header) time.Duration {
	if value := header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
			return min(time.Duration(seconds)*time.Second, policy.MaxBackoff)
		}
		if at, err := http.ParseTime(value); err == nil {
			return min(max(time.Until(at), 0), policy.MaxBackoff)
		}
	}
	delay := policy.InitialBackoff
	for index := 1; index < attempt && delay < policy.MaxBackoff; index++ {
		delay = min(delay*2, policy.MaxBackoff)
	}
	return delay
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
