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
	value := result.Report{SchemaVersion: result.SchemaVersion}
	require.NoError(t, Write(&output, FormatJSON, value))
	var decoded result.Report
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	assert.Equal(t, result.SchemaVersion, decoded.SchemaVersion)
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
