package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"version"}, &stdout, &stderr, emptyEnvironment, BuildInfo{Version: "v1.2.3", Commit: "abc123", Date: "2026-01-02"})
	assert.Equal(t, ExitOK, exitCode)
	assert.Contains(t, stdout.String(), "v1.2.3")
	assert.Contains(t, stdout.String(), "abc123")
	assert.Empty(t, stderr.String())
}

func TestRunRequiresAPIKeyEnvironment(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	environment := mapEnvironment(map[string]string{
		"AI_GATEWAY_BASE_URL": "https://gateway.example/v1",
		"AI_GATEWAY_MODEL":    "test-model",
	})
	exitCode := Run(context.Background(), []string{"run"}, &stdout, &stderr, environment, BuildInfo{})
	assert.Equal(t, ExitConfiguration, exitCode)
	assert.Contains(t, stderr.String(), "AI_GATEWAY_API_KEY")
}

func TestRunRejectsInvalidFormatBeforeNetworkCall(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	environment := mapEnvironment(map[string]string{"AI_GATEWAY_API_KEY": "secret"})
	exitCode := Run(context.Background(), []string{"run", "--format", "xml"}, &stdout, &stderr, environment, BuildInfo{})
	assert.Equal(t, ExitConfiguration, exitCode)
	assert.Contains(t, stderr.String(), "invalid --format")
}

func TestCatalogValidate(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"catalog", "validate"}, &stdout, &stderr, emptyEnvironment, BuildInfo{})
	assert.Equal(t, ExitOK, exitCode)
	assert.Contains(t, stdout.String(), "14 cases")
	assert.Contains(t, stdout.String(), "10 profiles")
	assert.Empty(t, stderr.String())
}

func TestProfilesListsReadinessClaims(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"profiles"}, &stdout, &stderr, emptyEnvironment, BuildInfo{})
	assert.Equal(t, ExitOK, exitCode)
	assert.Contains(t, stdout.String(), "codex-ready")
	assert.Contains(t, stdout.String(), "claude-code-ready")
}

func TestRunRejectsInvalidRetryPolicyBeforeNetworkCall(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	environment := mapEnvironment(map[string]string{
		"AI_GATEWAY_BASE_URL": "https://gateway.example/v1",
		"AI_GATEWAY_MODEL":    "test-model",
		"AI_GATEWAY_API_KEY":  "secret",
	})
	exitCode := Run(context.Background(), []string{"run", "--retries", "11"}, &stdout, &stderr, environment, BuildInfo{})
	assert.Equal(t, ExitConfiguration, exitCode)
	assert.Contains(t, stderr.String(), "retries must be between 0 and 10")
}

func emptyEnvironment(string) (string, bool) {
	return "", false
}

func mapEnvironment(values map[string]string) Environment {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
