package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestWriteText(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	value := result.Report{
		Target:    result.Target{Name: "gateway", Endpoints: map[string]result.TargetEndpoint{"openai": {BaseURL: "https://gateway.example/v1", Model: "test-model"}}},
		Summary:   result.Summary{Total: 1, Passed: 1},
		Scenarios: []result.Scenario{{ID: "OAI-MODL-001", Status: testcase.StatusPass}},
	}
	require.NoError(t, Write(&output, FormatText, value))
	assert.Contains(t, output.String(), "1 passed, 0 failed, 0 errors")
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	value := result.Report{SchemaURL: result.SchemaURL("unknown"), SchemaVersion: result.SchemaVersion, Build: result.Build{Commit: "unknown"}}
	require.NoError(t, Write(&output, FormatJSON, value))
	var decoded result.Report
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	assert.Equal(t, result.SchemaVersion, decoded.SchemaVersion)
}

func TestDecodeRejectsSchemaURLThatDoesNotMatchBuild(t *testing.T) {
	t.Parallel()
	input := `{"$schema":"https://example.com/report.schema.json","schema_version":"1.1.0","build":{"commit":"unknown"}}`
	_, err := Decode(bytes.NewBufferString(input))
	assert.ErrorContains(t, err, "does not match build commit")
}

func TestSanitizeRemovesShareSensitiveFields(t *testing.T) {
	t.Parallel()
	value := result.Report{
		Target:    result.Target{Endpoints: map[string]result.TargetEndpoint{"openai": {BaseURL: "https://private.example/v1", Model: "private-model"}}},
		Scenarios: []result.Scenario{{Evidence: map[string]any{"output": "private"}, Assertions: []result.Assertion{{Message: "detail", Expected: "secret", Observed: "value"}}}},
	}

	sanitized := Sanitize(value)

	assert.Empty(t, sanitized.Target.Endpoints["openai"].BaseURL)
	assert.Empty(t, sanitized.Target.Endpoints["openai"].Model)
	assert.Nil(t, sanitized.Scenarios[0].Evidence)
	assert.Empty(t, sanitized.Scenarios[0].Assertions[0].Message)
}

func TestAuthenticationExecutionPolicySurvivesReportProcessing(t *testing.T) {
	value := result.Report{SchemaURL: result.SchemaURL("unknown"), SchemaVersion: result.SchemaVersion, Runtime: result.Runtime{SkipAuthChecks: true}, Build: result.Build{Commit: "unknown"}}
	var encoded bytes.Buffer
	require.NoError(t, Write(&encoded, FormatJSON, Sanitize(value)))
	decoded, err := Decode(&encoded)
	require.NoError(t, err)
	assert.True(t, decoded.Runtime.SkipAuthChecks)
	var text bytes.Buffer
	require.NoError(t, Write(&text, FormatText, decoded))
	assert.Contains(t, text.String(), "skip-auth-checks")
	var rendered bytes.Buffer
	require.NoError(t, WriteHTML(&rendered, decoded))
	assert.Contains(t, rendered.String(), "skip-auth-checks")
}

func TestDecodePreviousSchemaVersions(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.1.0"} {
		value := result.Report{SchemaURL: result.SchemaURL("unknown"), SchemaVersion: version, Build: result.Build{Commit: "unknown"}}
		var encoded bytes.Buffer
		require.NoError(t, Write(&encoded, FormatJSON, value))
		decoded, err := Decode(&encoded)
		require.NoError(t, err)
		assert.False(t, decoded.Runtime.SkipAuthChecks)
	}
}
