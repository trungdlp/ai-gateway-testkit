package report

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

//go:embed report.html.tmpl assets/bootstrap.min.css assets/report.css assets/report.js
var htmlAssets embed.FS

type htmlReport struct {
	Title              string
	TargetName         string
	Fingerprint        string
	OverallStatus      string
	OverallLabel       string
	OverallHeadline    string
	OverallDescription string
	PassPercentage     int
	FailedPercentage   int
	UnavailablePercent int
	FailedOffset       int
	UnavailableOffset  int
	CoveragePercentage int
	Passed             int
	Failed             int
	Unavailable        int
	Total              int
	ProfilePasses      int
	ProfileTotal       int
	StartedAt          string
	Duration           string
	RunID              string
	CatalogVersion     string
	CatalogDigest      string
	SchemaVersion      string
	BuildVersion       string
	BuildCommit        string
	GoVersion          string
	Platform           string
	RawHTTPAttempts    int64
	RawHTTPRetries     int64
	SharingMode        bool
	Endpoints          []htmlEndpoint
	Profiles           []htmlProfile
	Scenarios          []htmlScenario
	Issues             []htmlIssue
	CSS                template.CSS
	Script             template.JS
	ContentPolicy      string
}

type htmlEndpoint struct {
	Protocol   string
	BaseURL    string
	Model      string
	APIVersion string
}

type htmlProfile struct {
	ID                 string
	Title              string
	Version            string
	Verdict            string
	VerdictLabel       string
	SuccessPercentage  int
	CoveragePercentage int
	Passed             int
	Required           int
	IssueCount         int
}

type htmlScenario struct {
	ID              string
	Title           string
	Revision        int
	Layer           string
	Suite           string
	Area            string
	Status          string
	StatusLabel     string
	Duration        string
	SearchText      string
	IssueCount      int
	Assertions      []htmlAssertion
	Evidence        string
	HasEvidence     bool
	DefaultExpanded bool
}

type htmlAssertion struct {
	ID            string
	Title         string
	Requirement   string
	Impact        string
	Status        string
	StatusLabel   string
	ReasonCode    string
	Message       string
	Expected      string
	Observed      string
	BaselineState string
	HasDetails    bool
}

type htmlIssue struct {
	AssertionID string
	Title       string
	Status      string
	StatusLabel string
	ReasonCode  string
	Message     string
}

// WriteHTML renders a standalone, offline HTML presentation of a canonical report.
func WriteHTML(w io.Writer, value result.Report) error {
	bootstrapCSS, err := htmlAssets.ReadFile("assets/bootstrap.min.css")
	if err != nil {
		return fmt.Errorf("read Bootstrap CSS: %w", err)
	}
	css, err := htmlAssets.ReadFile("assets/report.css")
	if err != nil {
		return fmt.Errorf("read HTML report CSS: %w", err)
	}
	script, err := htmlAssets.ReadFile("assets/report.js")
	if err != nil {
		return fmt.Errorf("read HTML report script: %w", err)
	}
	templateSource, err := htmlAssets.ReadFile("report.html.tmpl")
	if err != nil {
		return fmt.Errorf("read HTML report template: %w", err)
	}

	tmpl, err := template.New("report").Parse(string(templateSource))
	if err != nil {
		return fmt.Errorf("parse HTML report template: %w", err)
	}
	digest := sha256.Sum256(script)
	view := newHTMLReport(value)
	view.CSS = template.CSS(string(bootstrapCSS) + "\n" + string(css)) // Static, embedded application CSS.
	view.Script = template.JS(script)                                  // Static, embedded application JavaScript.
	view.ContentPolicy = "default-src 'none'; base-uri 'none'; connect-src 'none'; font-src data:; form-action 'none'; frame-ancestors 'none'; img-src data:; object-src 'none'; style-src 'unsafe-inline'; script-src 'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
	if err := tmpl.Execute(w, view); err != nil {
		return fmt.Errorf("render HTML report: %w", err)
	}
	return nil
}

func newHTMLReport(value result.Report) htmlReport {
	failed := value.Summary.Failed
	unavailable := value.Summary.Errors + value.Summary.Blocked + value.Summary.Skipped + value.Summary.NotApplicable
	evaluated := value.Summary.Passed + value.Summary.Failed
	view := htmlReport{
		Title:              "AI Gateway Compatibility Report",
		TargetName:         value.Target.Name,
		Fingerprint:        value.Target.Fingerprint,
		Passed:             value.Summary.Passed,
		Failed:             failed,
		Unavailable:        unavailable,
		Total:              value.Summary.Total,
		PassPercentage:     ratioPercentage(value.Summary.Passed, value.Summary.Total),
		CoveragePercentage: ratioPercentage(evaluated, value.Summary.Total),
		StartedAt:          value.StartedAt.UTC().Format("02 Jan 2006, 15:04 UTC"),
		Duration:           formatDuration(value.DurationMS),
		RunID:              value.RunID,
		CatalogVersion:     value.CatalogVersion,
		CatalogDigest:      value.CatalogDigest,
		SchemaVersion:      value.SchemaVersion,
		BuildVersion:       value.Build.Version,
		BuildCommit:        value.Build.Commit,
		GoVersion:          value.Runtime.GoVersion,
		Platform:           value.Runtime.OS + "/" + value.Runtime.Arch,
		RawHTTPAttempts:    value.Runtime.Retry.RawHTTPAttempts,
		RawHTTPRetries:     value.Runtime.Retry.RawHTTPRetries,
	}
	view.FailedPercentage = ratioPercentage(failed, value.Summary.Total)
	view.UnavailablePercent = 100 - view.PassPercentage - view.FailedPercentage
	if view.UnavailablePercent < 0 {
		view.UnavailablePercent = 0
	}
	view.FailedOffset = view.PassPercentage
	view.UnavailableOffset = view.PassPercentage + view.FailedPercentage

	view.OverallStatus, view.OverallLabel, view.OverallHeadline, view.OverallDescription = overallCopy(value.Profiles)
	view.Endpoints, view.SharingMode = htmlEndpoints(value.Target.Endpoints)
	for _, profile := range value.Profiles {
		if profile.Verdict == result.VerdictPass {
			view.ProfilePasses++
		}
		view.Profiles = append(view.Profiles, htmlProfile{
			ID:                 profile.ID,
			Title:              profile.Title,
			Version:            profile.Version,
			Verdict:            strings.ToLower(string(profile.Verdict)),
			VerdictLabel:       string(profile.Verdict),
			SuccessPercentage:  int(profile.SuccessRatio*100 + 0.5),
			CoveragePercentage: int(profile.CoverageRatio*100 + 0.5),
			Passed:             profile.Passed,
			Required:           profile.Required,
			IssueCount:         profile.Failed + profile.Indeterminate,
		})
	}
	view.ProfileTotal = len(value.Profiles)
	sort.SliceStable(view.Profiles, func(i, j int) bool {
		leftPriority := verdictPriority(view.Profiles[i].Verdict)
		rightPriority := verdictPriority(view.Profiles[j].Verdict)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return view.Profiles[i].Title < view.Profiles[j].Title
	})

	for _, scenario := range value.Scenarios {
		item := htmlScenario{
			ID:              scenario.ID,
			Title:           scenario.Title,
			Revision:        scenario.Revision,
			Layer:           string(scenario.Layer),
			Suite:           scenario.Suite,
			Area:            scenario.Area,
			Status:          string(scenario.Status),
			StatusLabel:     statusLabel(scenario.Status),
			Duration:        formatDuration(scenario.DurationMS),
			DefaultExpanded: scenario.Status != testcase.StatusPass,
		}
		searchParts := []string{scenario.ID, scenario.Title, string(scenario.Status), string(scenario.Layer), scenario.Suite, scenario.Area}
		for _, assertion := range scenario.Assertions {
			hasDetails := assertion.ReasonCode != "" || assertion.Message != "" || assertion.Expected != "" || assertion.Observed != "" || assertion.BaselineState != ""
			assertionView := htmlAssertion{
				ID:            assertion.ID,
				Title:         assertion.Title,
				Requirement:   string(assertion.Requirement),
				Impact:        string(assertion.Impact),
				Status:        string(assertion.Status),
				StatusLabel:   statusLabel(assertion.Status),
				ReasonCode:    assertion.ReasonCode,
				Message:       assertion.Message,
				Expected:      assertion.Expected,
				Observed:      assertion.Observed,
				BaselineState: assertion.BaselineState,
				HasDetails:    hasDetails,
			}
			item.Assertions = append(item.Assertions, assertionView)
			searchParts = append(searchParts, assertion.ID, assertion.Title, assertion.ReasonCode, assertion.Message)
			if assertion.Status != testcase.StatusPass {
				item.IssueCount++
				view.Issues = append(view.Issues, htmlIssue{
					AssertionID: assertion.ID,
					Title:       assertion.Title,
					Status:      string(assertion.Status),
					StatusLabel: statusLabel(assertion.Status),
					ReasonCode:  assertion.ReasonCode,
					Message:     assertion.Message,
				})
			}
		}
		if len(scenario.Evidence) > 0 {
			if encoded, err := json.MarshalIndent(scenario.Evidence, "", "  "); err == nil {
				item.Evidence = string(encoded)
				item.HasEvidence = true
			}
		}
		item.SearchText = strings.ToLower(strings.Join(searchParts, " "))
		view.Scenarios = append(view.Scenarios, item)
	}
	return view
}

func verdictPriority(verdict string) int {
	switch verdict {
	case "fail":
		return 0
	case "indeterminate":
		return 1
	default:
		return 2
	}
}

func overallCopy(profiles []result.Profile) (status, label, headline, description string) {
	status = "pass"
	label = "Ready"
	headline = "Compatibility verified"
	description = "Every selected compatibility claim passed with complete, usable evidence."
	for _, profile := range profiles {
		if profile.Verdict == result.VerdictFail {
			return "fail", "Action required", "Compatibility gap detected", "The gateway is broadly operational, with one or more precise contract gaps to resolve."
		}
		if profile.Verdict == result.VerdictIndeterminate {
			status = "indeterminate"
			label = "Review evidence"
			headline = "Compatibility is not yet conclusive"
			description = "No confirmed incompatibility was found, but some required evidence was unavailable."
		}
	}
	return status, label, headline, description
}

func htmlEndpoints(endpoints map[string]result.TargetEndpoint) ([]htmlEndpoint, bool) {
	protocols := make([]string, 0, len(endpoints))
	sharingMode := len(endpoints) > 0
	for protocol, endpoint := range endpoints {
		protocols = append(protocols, protocol)
		if endpoint.BaseURL != "" || endpoint.Model != "" {
			sharingMode = false
		}
	}
	sort.Strings(protocols)
	items := make([]htmlEndpoint, 0, len(protocols))
	for _, protocol := range protocols {
		endpoint := endpoints[protocol]
		items = append(items, htmlEndpoint{
			Protocol:   protocol,
			BaseURL:    endpoint.BaseURL,
			Model:      endpoint.Model,
			APIVersion: endpoint.APIVersion,
		})
	}
	return items, sharingMode
}

func statusLabel(status testcase.Status) string {
	switch status {
	case testcase.StatusPass:
		return "Passed"
	case testcase.StatusFail:
		return "Failed"
	case testcase.StatusError:
		return "Error"
	case testcase.StatusBlocked:
		return "Blocked"
	case testcase.StatusSkipped:
		return "Skipped"
	case testcase.StatusNotApplicable:
		return "Not applicable"
	default:
		return string(status)
	}
}

func ratioPercentage(numerator, denominator int) int {
	if denominator == 0 {
		return 0
	}
	return int(float64(numerator)/float64(denominator)*100 + 0.5)
}

func formatDuration(milliseconds int64) string {
	if milliseconds < 1000 {
		return fmt.Sprintf("%d ms", milliseconds)
	}
	duration := time.Duration(milliseconds) * time.Millisecond
	if duration < time.Minute {
		return fmt.Sprintf("%.1f s", duration.Seconds())
	}
	return duration.Round(time.Second).String()
}
