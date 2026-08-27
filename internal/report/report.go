package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

func Write(w io.Writer, format string, value result.Report) error {
	switch format {
	case FormatText:
		return writeText(w, value)
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func Decode(reader io.Reader) (result.Report, error) {
	var value result.Report
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return result.Report{}, err
	}
	if value.SchemaVersion != result.SchemaVersion {
		return result.Report{}, fmt.Errorf("unsupported report schema version %q", value.SchemaVersion)
	}
	return value, nil
}

func Sanitize(value result.Report) result.Report {
	for protocol, endpoint := range value.Target.Endpoints {
		endpoint.BaseURL = ""
		endpoint.Model = ""
		value.Target.Endpoints[protocol] = endpoint
	}
	for scenarioIndex := range value.Scenarios {
		value.Scenarios[scenarioIndex].Evidence = nil
		for assertionIndex := range value.Scenarios[scenarioIndex].Assertions {
			assertion := &value.Scenarios[scenarioIndex].Assertions[assertionIndex]
			assertion.Message = ""
			assertion.Expected = ""
			assertion.Observed = ""
		}
	}
	return value
}

func writeText(w io.Writer, value result.Report) error {
	if _, err := fmt.Fprintf(w, "Target:  %s (%s)\nCatalog: %s\nRun:     %s\n\n", value.Target.Name, value.Target.Fingerprint, value.CatalogVersion, value.RunID); err != nil {
		return err
	}
	protocols := make([]string, 0, len(value.Target.Endpoints))
	for protocol := range value.Target.Endpoints {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)
	for _, protocol := range protocols {
		endpoint := value.Target.Endpoints[protocol]
		if _, err := fmt.Fprintf(w, "%-10s %s  model=%s\n", protocol+":", endpoint.BaseURL, endpoint.Model); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Retries:   %d across %d raw HTTP attempts (max %d per request)\n", value.Runtime.Retry.RawHTTPRetries, value.Runtime.Retry.RawHTTPAttempts, value.Runtime.Retry.MaxRetries); err != nil {
		return err
	}
	if len(value.Profiles) > 0 {
		if _, err := fmt.Fprintln(w, "\nProfiles:"); err != nil {
			return err
		}
		for _, profile := range value.Profiles {
			if _, err := fmt.Fprintf(w, "  %-24s %-13s coverage=%s success=%s\n", profile.ID, profile.Verdict, percentage(profile.CoverageRatio), percentage(profile.SuccessRatio)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(w, "\nScenarios:"); err != nil {
		return err
	}
	for _, scenario := range value.Scenarios {
		if _, err := fmt.Fprintf(w, "  %-7s %-16s %4dms  %s\n", strings.ToUpper(string(scenario.Status)), scenario.ID, scenario.DurationMS, scenario.Title); err != nil {
			return err
		}
		for _, assertion := range scenario.Assertions {
			if assertion.Status == testcase.StatusPass {
				continue
			}
			if _, err := fmt.Fprintf(w, "          %-14s %-20s %s", strings.ToUpper(string(assertion.Status)), assertion.ID, assertion.ReasonCode); err != nil {
				return err
			}
			if assertion.Message != "" {
				if _, err := fmt.Fprintf(w, ": %s", assertion.Message); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "\nSummary: %d passed, %d failed, %d errors, %d blocked, %d skipped, %d not applicable (%d total, %dms)\n", value.Summary.Passed, value.Summary.Failed, value.Summary.Errors, value.Summary.Blocked, value.Summary.Skipped, value.Summary.NotApplicable, value.Summary.Total, value.DurationMS)
	return err
}

func percentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}
