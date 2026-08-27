package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/report"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
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

func TestReportRenderSanitizesByDefault(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "report.json")
	shareablePath := filepath.Join(directory, "shareable.html")
	fullPath := filepath.Join(directory, "full.html")
	value := result.Report{
		SchemaURL:      result.SchemaURL("unknown"),
		SchemaVersion:  result.SchemaVersion,
		CatalogVersion: "2026-08-01",
		CatalogDigest:  "sha256:" + strings.Repeat("a", 64),
		RunID:          "run-test",
		StartedAt:      time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
		Build:          result.Build{Version: "dev", Commit: "unknown"},
		Target: result.Target{
			Name: "gateway", Fingerprint: "sha256:" + strings.Repeat("b", 64),
			Endpoints: map[string]result.TargetEndpoint{"openai": {BaseURL: "https://private.example/v1", Model: "private-model"}},
		},
		Profiles: []result.Profile{{ID: "oai-core", Title: "OpenAI Core", Version: "1.0.0", Verdict: result.VerdictFail}},
		Summary:  result.Summary{Total: 1, Failed: 1},
		Scenarios: []result.Scenario{{
			ID: "OAI-RESP-001", Revision: 1, Title: "Responses", Layer: testcase.LayerProtocol, Status: testcase.StatusFail,
			Assertions: []result.Assertion{{ID: "OAI-RESP-001/A01", Title: "Response is valid", Requirement: testcase.Must, Impact: testcase.Blocker, Status: testcase.StatusFail, Message: "private diagnostic"}},
		}},
	}
	var encoded bytes.Buffer
	require.NoError(t, report.Write(&encoded, report.FormatJSON, value))
	require.NoError(t, os.WriteFile(inputPath, encoded.Bytes(), 0o600))

	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), []string{"report", "render", inputPath, "--output", shareablePath}, &stdout, &stderr, emptyEnvironment, BuildInfo{})
	require.Equal(t, ExitOK, exitCode, stderr.String())
	shareable, err := os.ReadFile(shareablePath)
	require.NoError(t, err)
	assert.NotContains(t, string(shareable), "https://private.example/v1")
	assert.NotContains(t, string(shareable), "private-model")
	assert.NotContains(t, string(shareable), "private diagnostic")
	assert.Contains(t, string(shareable), "Share-safe")

	stdout.Reset()
	stderr.Reset()
	exitCode = Run(context.Background(), []string{"report", "render", inputPath, "--output", fullPath, "--include-sensitive-details"}, &stdout, &stderr, emptyEnvironment, BuildInfo{})
	require.Equal(t, ExitOK, exitCode, stderr.String())
	full, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Contains(t, string(full), "https://private.example/v1")
	assert.Contains(t, string(full), "private-model")
	assert.Contains(t, string(full), "private diagnostic")
	assert.Contains(t, string(full), "Full details")
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
