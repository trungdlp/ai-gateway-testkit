package engine

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/target"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const invalidCredential = "agtk-invalid-credential"

type liveEnvironment struct {
	targets target.Resolved
	clients map[string]*gateway.Client
	secrets []string
	agent   testcase.AgentRunner
}

func newLiveEnvironment(targets target.Resolved, agent testcase.AgentRunner) *liveEnvironment {
	environment := &liveEnvironment{targets: targets, clients: map[string]*gateway.Client{}, secrets: []string{invalidCredential}, agent: agent}
	for protocol, endpoint := range targets.Endpoints {
		environment.clients[protocol] = gateway.NewClientWithRetry(endpoint.BaseURL, endpoint.Timeout, endpoint.Retry)
		environment.secrets = append(environment.secrets, endpoint.Credential)
	}
	return environment
}

func (e *liveEnvironment) RetryStats() gateway.Stats {
	var total gateway.Stats
	for _, client := range e.clients {
		stats := client.Stats()
		total.Requests += stats.Requests
		total.Attempts += stats.Attempts
		total.Retries += stats.Retries
		total.Exhausted += stats.Exhausted
	}
	return total
}

func (e *liveEnvironment) RunAgent(ctx context.Context, request testcase.AgentRequest) (testcase.AgentOutcome, error) {
	if e.agent == nil {
		return testcase.AgentOutcome{Available: false}, nil
	}
	target, ok := e.Target(request.Protocol)
	if !ok {
		return testcase.AgentOutcome{Available: false}, nil
	}
	return e.agent.Run(ctx, request, target)
}

func (e *liveEnvironment) Target(protocol string) (testcase.Target, bool) {
	target, ok := e.targets.Endpoints[protocol]
	return target, ok
}

func (e *liveEnvironment) DoJSON(ctx context.Context, request testcase.Request) (gateway.Response, error) {
	endpoint, ok := e.targets.Endpoints[request.Protocol]
	if !ok {
		return gateway.Response{}, fmt.Errorf("target protocol %q is not configured", request.Protocol)
	}
	headers := request.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if request.Protocol == "anthropic" {
		version := endpoint.APIVersion
		if version == "" {
			version = "2023-06-01"
		}
		headers.Set("Anthropic-Version", version)
	}
	credential := endpoint.Credential
	if request.Auth == testcase.AuthInvalid {
		credential = invalidCredential
	}
	if request.Auth != testcase.AuthNone {
		if request.Protocol == "anthropic" {
			headers.Set("X-API-Key", credential)
		} else {
			headers.Set("Authorization", "Bearer "+credential)
		}
	}
	return e.clients[request.Protocol].DoJSON(ctx, request.Method, request.Path, request.Body, headers)
}

func (e *liveEnvironment) Redact(value string) string {
	for _, secret := range e.secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
