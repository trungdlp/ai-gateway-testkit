package testcase

import (
	"context"
	"net/http"
	"time"

	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
)

type Layer string

const (
	LayerProtocol   Layer = "protocol"
	LayerSDK        Layer = "sdk"
	LayerBehavioral Layer = "behavioral"
	LayerAgent      Layer = "agent"
	LayerSecurity   Layer = "security"
	LayerResilience Layer = "resilience"
)

type Stability string

const (
	StabilityExperimental Stability = "experimental"
	StabilityStable       Stability = "stable"
	StabilityDeprecated   Stability = "deprecated"
	StabilityRetired      Stability = "retired"
)

type Determinism string

const (
	Deterministic Determinism = "deterministic"
	Probabilistic Determinism = "probabilistic"
	Operational   Determinism = "operational"
)

type Requirement string

const (
	Must   Requirement = "must"
	Should Requirement = "should"
	May    Requirement = "may"
)

type Impact string

const (
	Blocker Impact = "blocker"
	High    Impact = "high"
	Medium  Impact = "medium"
	Low     Impact = "low"
)

type Status string

const (
	StatusPass          Status = "pass"
	StatusFail          Status = "fail"
	StatusError         Status = "error"
	StatusBlocked       Status = "blocked"
	StatusSkipped       Status = "skipped"
	StatusNotApplicable Status = "not_applicable"
)

type AuthMode string

const (
	AuthNone    AuthMode = "none"
	AuthValid   AuthMode = "valid"
	AuthInvalid AuthMode = "invalid"
)

type SpecReference struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Section string `json:"section,omitempty"`
}

type Cost struct {
	Requests        int `json:"requests"`
	MaxInputTokens  int `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
}

type Risk struct {
	Mutation        string `json:"mutation"`
	CleanupRequired bool   `json:"cleanup_required"`
	ContainsOutput  bool   `json:"contains_model_output"`
}

type AssertionDefinition struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Requirement Requirement `json:"requirement"`
	Impact      Impact      `json:"impact"`
}

type Definition struct {
	ID             string                `json:"id"`
	Revision       int                   `json:"revision"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	Layer          Layer                 `json:"layer"`
	Suite          string                `json:"suite"`
	Area           string                `json:"area"`
	Stability      Stability             `json:"stability"`
	Determinism    Determinism           `json:"determinism"`
	SpecReferences []SpecReference       `json:"spec_references,omitempty"`
	Preconditions  []string              `json:"preconditions,omitempty"`
	DependsOn      []string              `json:"depends_on,omitempty"`
	Cost           Cost                  `json:"cost"`
	Risk           Risk                  `json:"risk"`
	Assertions     []AssertionDefinition `json:"assertions"`
	Run            RunFunc               `json:"-"`
}

type Target struct {
	Protocol   string              `json:"protocol"`
	BaseURL    string              `json:"base_url"`
	Model      string              `json:"model"`
	Credential string              `json:"-"`
	Timeout    time.Duration       `json:"-"`
	APIVersion string              `json:"api_version,omitempty"`
	Retry      gateway.RetryPolicy `json:"-"`
}

type Request struct {
	Protocol string
	Method   string
	Path     string
	Body     any
	Auth     AuthMode
	Headers  http.Header
}

type Environment interface {
	Target(protocol string) (Target, bool)
	DoJSON(context.Context, Request) (gateway.Response, error)
	RunAgent(context.Context, AgentRequest) (AgentOutcome, error)
	Redact(string) string
}

type AgentRequest struct {
	Agent    string
	Protocol string
	Prompt   string
}

type AgentOutcome struct {
	Available        bool
	ExitCode         int
	Output           string
	ResultFile       string
	ValidationMarker bool
}

type AgentRunner interface {
	Run(context.Context, AgentRequest, Target) (AgentOutcome, error)
}

type AssertionResult struct {
	ID         string `json:"id"`
	Status     Status `json:"status"`
	ReasonCode string `json:"reason_code,omitempty"`
	Message    string `json:"message,omitempty"`
	Expected   string `json:"expected,omitempty"`
	Observed   string `json:"observed,omitempty"`
}

type Execution struct {
	Assertions []AssertionResult `json:"assertions"`
	Evidence   map[string]any    `json:"evidence,omitempty"`
}

type RunFunc func(context.Context, Environment) Execution

func Pass(id string) AssertionResult {
	return AssertionResult{ID: id, Status: StatusPass}
}

func Fail(id, reason, message string) AssertionResult {
	return AssertionResult{ID: id, Status: StatusFail, ReasonCode: reason, Message: message}
}

func Error(id, reason, message string) AssertionResult {
	return AssertionResult{ID: id, Status: StatusError, ReasonCode: reason, Message: message}
}

func Blocked(id, reason, message string) AssertionResult {
	return AssertionResult{ID: id, Status: StatusBlocked, ReasonCode: reason, Message: message}
}

func NotApplicable(id, message string) AssertionResult {
	return AssertionResult{ID: id, Status: StatusNotApplicable, ReasonCode: "TARGET.NOT_CONFIGURED", Message: message}
}

func Result(id string, passed bool, reason, message string) AssertionResult {
	if passed {
		return Pass(id)
	}
	return Fail(id, reason, message)
}
