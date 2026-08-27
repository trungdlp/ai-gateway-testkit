package result

import (
	"regexp"
	"strings"
	"time"

	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const (
	SchemaVersion       = "1.1.0"
	reportSchemaBaseURL = "https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit"
)

var fullCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func SchemaURL(commit string) string {
	commit = strings.TrimSpace(commit)
	if !fullCommitPattern.MatchString(commit) {
		commit = "main"
	}
	return reportSchemaBaseURL + "/" + commit + "/schemas/report.schema.json"
}

type Build struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type Runtime struct {
	OS                 string            `json:"os"`
	Arch               string            `json:"arch"`
	GoVersion          string            `json:"go_version"`
	DependencyVersions map[string]string `json:"dependency_versions"`
	Retry              Retry             `json:"retry"`
}

type Retry struct {
	MaxRetries       int   `json:"max_retries"`
	InitialBackoffMS int64 `json:"initial_backoff_ms"`
	MaxBackoffMS     int64 `json:"max_backoff_ms"`
	RawHTTPRequests  int64 `json:"raw_http_requests"`
	RawHTTPAttempts  int64 `json:"raw_http_attempts"`
	RawHTTPRetries   int64 `json:"raw_http_retries"`
	Exhausted        int64 `json:"exhausted"`
}

type Target struct {
	Name        string                    `json:"name"`
	Fingerprint string                    `json:"fingerprint"`
	Endpoints   map[string]TargetEndpoint `json:"endpoints"`
}

type TargetEndpoint struct {
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	APIVersion string `json:"api_version,omitempty"`
}

type Assertion struct {
	ID            string               `json:"id"`
	Title         string               `json:"title"`
	Requirement   testcase.Requirement `json:"requirement"`
	Impact        testcase.Impact      `json:"impact"`
	Status        testcase.Status      `json:"status"`
	ReasonCode    string               `json:"reason_code,omitempty"`
	Message       string               `json:"message,omitempty"`
	Expected      string               `json:"expected,omitempty"`
	Observed      string               `json:"observed,omitempty"`
	BaselineState string               `json:"baseline_state,omitempty"`
}

type Scenario struct {
	ID         string          `json:"id"`
	Revision   int             `json:"revision"`
	Title      string          `json:"title"`
	Layer      testcase.Layer  `json:"layer"`
	Suite      string          `json:"suite"`
	Area       string          `json:"area"`
	Status     testcase.Status `json:"status"`
	DurationMS int64           `json:"duration_ms"`
	Assertions []Assertion     `json:"assertions"`
	Evidence   map[string]any  `json:"evidence,omitempty"`
}

type Verdict string

const (
	VerdictPass          Verdict = "PASS"
	VerdictFail          Verdict = "FAIL"
	VerdictIndeterminate Verdict = "INDETERMINATE"
)

type Profile struct {
	ID               string   `json:"id"`
	Version          string   `json:"version"`
	Title            string   `json:"title"`
	Verdict          Verdict  `json:"verdict"`
	Required         int      `json:"required"`
	Evaluated        int      `json:"evaluated"`
	Passed           int      `json:"passed"`
	Failed           int      `json:"failed"`
	Indeterminate    int      `json:"indeterminate"`
	SuccessRatio     float64  `json:"success_ratio"`
	CoverageRatio    float64  `json:"coverage_ratio"`
	FailureIDs       []string `json:"failure_ids,omitempty"`
	IndeterminateIDs []string `json:"indeterminate_ids,omitempty"`
}

type Summary struct {
	Total         int `json:"total"`
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Errors        int `json:"errors"`
	Blocked       int `json:"blocked"`
	Skipped       int `json:"skipped"`
	NotApplicable int `json:"not_applicable"`
}

type Report struct {
	SchemaURL        string     `json:"$schema"`
	SchemaVersion    string     `json:"schema_version"`
	CatalogVersion   string     `json:"catalog_version"`
	CatalogDigest    string     `json:"catalog_digest"`
	RunID            string     `json:"run_id"`
	StartedAt        time.Time  `json:"started_at"`
	DurationMS       int64      `json:"duration_ms"`
	Build            Build      `json:"build"`
	Runtime          Runtime    `json:"runtime"`
	Target           Target     `json:"target"`
	SelectedProfiles []string   `json:"selected_profiles"`
	Profiles         []Profile  `json:"profiles"`
	Summary          Summary    `json:"summary"`
	Scenarios        []Scenario `json:"scenarios"`
}
