package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"html"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestWriteHTMLRendersStandaloneInteractiveReport(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	reportValue := htmlTestReport()
	require.NoError(t, WriteHTML(&output, reportValue))
	rendered := output.String()

	assert.True(t, strings.HasPrefix(rendered, "<!doctype html>"))
	assert.Contains(t, rendered, "Compatibility gap detected")
	assert.Contains(t, rendered, "OAI-RESP-001/A01")
	assert.Contains(t, rendered, "SCHEMA.INVALID")
	assert.Contains(t, rendered, "Bootstrap  v5.3.8")
	assert.Contains(t, rendered, "Result distribution")
	assert.Contains(t, rendered, "class=\"stacked-chart\"")
	assert.Contains(t, rendered, "class=\"profile-chart\"")
	assert.Contains(t, rendered, "class=\"summary-icon status-pass\"")
	assert.Contains(t, rendered, "class=\"section-heading-icon icon-danger\"")
	assert.Contains(t, rendered, "id=\"scenario-search\"")
	assert.Contains(t, rendered, "@media print")
	assert.NotContains(t, rendered, "<script src=")
	assert.NotContains(t, rendered, "<link rel=\"stylesheet\"")
}

func TestWriteHTMLEscapesReportData(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	reportValue := htmlTestReport()
	reportValue.Target.Name = `gateway</title><script>alert("xss")</script>`
	reportValue.Scenarios[0].Assertions[0].Message = `<img src=x onerror=alert("xss")>`
	require.NoError(t, WriteHTML(&output, reportValue))
	rendered := output.String()

	assert.NotContains(t, rendered, `<script>alert("xss")</script>`)
	assert.NotContains(t, rendered, `<img src=x`)
	assert.Contains(t, rendered, `&lt;script&gt;alert`)
	assert.Contains(t, rendered, `&lt;img src=x`)
}

func TestWriteHTMLContentPolicyPinsEmbeddedScript(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	require.NoError(t, WriteHTML(&output, htmlTestReport()))
	rendered := output.String()
	metaPattern := regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]+)">`)
	scriptPattern := regexp.MustCompile(`(?s)<script>(.*)</script>`)
	metaMatch := metaPattern.FindStringSubmatch(rendered)
	scriptMatch := scriptPattern.FindStringSubmatch(rendered)
	require.Len(t, metaMatch, 2)
	require.Len(t, scriptMatch, 2)

	digest := sha256.Sum256([]byte(scriptMatch[1]))
	expected := "script-src 'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
	assert.Contains(t, html.UnescapeString(metaMatch[1]), expected)
}

func TestHTMLReportBuildsCompleteDistributionSegments(t *testing.T) {
	t.Parallel()

	value := htmlTestReport()
	value.Summary = result.Summary{Total: 3, Passed: 1, Failed: 1, Errors: 1}
	view := newHTMLReport(value)

	assert.Equal(t, 33, view.PassPercentage)
	assert.Equal(t, 33, view.FailedPercentage)
	assert.Equal(t, 34, view.UnavailablePercent)
	assert.Equal(t, 33, view.FailedOffset)
	assert.Equal(t, 66, view.UnavailableOffset)
}

func htmlTestReport() result.Report {
	return result.Report{
		SchemaVersion:  result.SchemaVersion,
		CatalogVersion: "2026-08-01",
		CatalogDigest:  "sha256:" + strings.Repeat("a", 64),
		RunID:          "run-test",
		StartedAt:      time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
		DurationMS:     1500,
		Build:          result.Build{Version: "dev", Commit: "unknown"},
		Runtime: result.Runtime{
			OS: "darwin", Arch: "arm64", GoVersion: "go1.27.0",
			Retry: result.Retry{RawHTTPAttempts: 2},
		},
		Target: result.Target{
			Name:        "test-gateway",
			Fingerprint: "sha256:" + strings.Repeat("b", 64),
			Endpoints:   map[string]result.TargetEndpoint{"openai": {BaseURL: "https://gateway.example/v1", Model: "test-model"}},
		},
		Profiles: []result.Profile{{
			ID: "oai-core", Version: "1.0.0", Title: "OpenAI Core", Verdict: result.VerdictFail,
			Required: 1, Evaluated: 1, Failed: 1, CoverageRatio: 1,
		}},
		Summary: result.Summary{Total: 1, Failed: 1},
		Scenarios: []result.Scenario{{
			ID: "OAI-RESP-001", Revision: 1, Title: "Responses", Layer: testcase.LayerProtocol,
			Suite: "openai", Area: "responses", Status: testcase.StatusFail, DurationMS: 120,
			Assertions: []result.Assertion{{
				ID: "OAI-RESP-001/A01", Title: "Response is valid", Requirement: testcase.Must,
				Impact: testcase.Blocker, Status: testcase.StatusFail, ReasonCode: "SCHEMA.INVALID", Message: "invalid response",
			}},
		}},
	}
}
