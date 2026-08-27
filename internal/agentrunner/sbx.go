package agentrunner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

type SBX struct {
	Executable string
}

func NewSBX() *SBX {
	return &SBX{Executable: "sbx"}
}

func (r *SBX) Run(ctx context.Context, request testcase.AgentRequest, target testcase.Target) (outcome testcase.AgentOutcome, runErr error) {
	if request.Agent != "codex" && request.Agent != "claude" {
		return outcome, fmt.Errorf("unsupported sbx agent %q", request.Agent)
	}
	if target.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, target.Timeout)
		defer cancel()
	}
	workspace, err := os.MkdirTemp("", "agtk-agent-*")
	if err != nil {
		return outcome, fmt.Errorf("create agent workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := writeFixture(workspace, request, target); err != nil {
		return outcome, err
	}
	environmentFile, err := os.CreateTemp("", "agtk-agent-env-*")
	if err != nil {
		return outcome, fmt.Errorf("create agent environment: %w", err)
	}
	environmentPath := environmentFile.Name()
	defer os.Remove(environmentPath)
	if err := environmentFile.Chmod(0o600); err != nil {
		environmentFile.Close()
		return outcome, fmt.Errorf("secure agent environment: %w", err)
	}
	name := "agtk-" + request.Agent + "-" + randomSuffix()
	created := false
	defer func() {
		if !created {
			return
		}
		cleanup := exec.Command(r.Executable, "rm", "--force", name)
		if output, err := cleanup.CombinedOutput(); err != nil && runErr == nil {
			runErr = fmt.Errorf("remove sbx sandbox %s: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
	}()

	output, err := exec.CommandContext(ctx, r.Executable, "create", "--quiet", "--name", name, request.Agent, workspace).CombinedOutput()
	if err != nil {
		return outcome, fmt.Errorf("create sbx sandbox: %w: %s", err, strings.TrimSpace(string(output)))
	}
	created = true
	outcome.Available = true
	placeholder, err := r.configureSecret(ctx, name, request, target)
	if err != nil {
		return outcome, err
	}
	if err := r.allowGatewayNetwork(ctx, name, target); err != nil {
		return outcome, err
	}
	if err := writeEnvironment(environmentFile, request, target); err != nil {
		environmentFile.Close()
		return outcome, err
	}
	if err := environmentFile.Close(); err != nil {
		return outcome, fmt.Errorf("close agent environment: %w", err)
	}

	pwdOutput, err := exec.CommandContext(ctx, r.Executable, "exec", name, "pwd").CombinedOutput()
	if err != nil {
		return outcome, fmt.Errorf("resolve sbx workspace: %w: %s", err, strings.TrimSpace(string(pwdOutput)))
	}
	containerWorkspace := lastNonEmptyLine(string(pwdOutput))
	if containerWorkspace == "" {
		return outcome, fmt.Errorf("resolve sbx workspace: empty path")
	}
	args := []string{"exec", "--env-file", environmentPath, "--env", credentialEnvironment(request) + "=" + placeholder}
	if request.Agent == "claude" {
		args = append(args, "--env", "ANTHROPIC_API_KEY=")
	}
	args = append(args, "--workdir", containerWorkspace, name)
	args = append(args, agentCommand(request, target)...)
	command := exec.CommandContext(ctx, r.Executable, args...)
	output, err = command.CombinedOutput()
	outcome.Output = string(output)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			outcome.ExitCode = exitErr.ExitCode()
		} else {
			return outcome, fmt.Errorf("execute agent in sbx: %w", err)
		}
	}
	resultData, _ := os.ReadFile(filepath.Join(workspace, "result.txt"))
	outcome.ResultFile = string(resultData)
	if _, err := os.Stat(filepath.Join(workspace, ".agtk-validated")); err == nil {
		outcome.ValidationMarker = true
	}
	return outcome, nil
}

func writeFixture(workspace string, request testcase.AgentRequest, target testcase.Target) error {
	task := "# Task\n\nCreate `result.txt` containing exactly `AGTK_AGENT_OK` followed by a newline. Then run `./verify.sh`.\n"
	if err := os.WriteFile(filepath.Join(workspace, "TASK.md"), []byte(task), 0o600); err != nil {
		return fmt.Errorf("write agent task: %w", err)
	}
	validator := "#!/bin/sh\nset -eu\ntest \"$(cat result.txt)\" = \"AGTK_AGENT_OK\"\n: > .agtk-validated\n"
	if err := os.WriteFile(filepath.Join(workspace, "verify.sh"), []byte(validator), 0o700); err != nil {
		return fmt.Errorf("write agent validator: %w", err)
	}
	return nil
}

func writeEnvironment(file *os.File, request testcase.AgentRequest, target testcase.Target) error {
	values := []string{"AGTK_TARGET_MODEL=" + target.Model}
	if request.Protocol == "openai" {
		values = append(values, "OPENAI_BASE_URL="+target.BaseURL)
	} else {
		values = append(values, "ANTHROPIC_BASE_URL="+target.BaseURL)
	}
	if _, err := file.WriteString(strings.Join(values, "\n") + "\n"); err != nil {
		return fmt.Errorf("write agent environment: %w", err)
	}
	return nil
}

func (r *SBX) configureSecret(ctx context.Context, sandbox string, request testcase.AgentRequest, target testcase.Target) (string, error) {
	endpoint, err := url.Parse(target.BaseURL)
	if err != nil || endpoint.Hostname() == "" {
		return "", fmt.Errorf("parse gateway host for sandbox secret")
	}
	environmentName := credentialEnvironment(request)
	command := exec.CommandContext(ctx, r.Executable, "secret", "set-custom", "--sandbox", sandbox, "--host", endpoint.Hostname(), "--env", environmentName, "--value", target.Credential)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("configure scoped sbx gateway secret: %w: %s", err, strings.TrimSpace(string(output)))
	}
	placeholder := regexp.MustCompile(`sbx-cs-[A-Za-z0-9]+`).FindString(string(output))
	if placeholder == "" {
		return "", fmt.Errorf("configure scoped sbx gateway secret: placeholder was not returned")
	}
	return placeholder, nil
}

func credentialEnvironment(request testcase.AgentRequest) string {
	if request.Agent == "claude" {
		return "ANTHROPIC_AUTH_TOKEN"
	}
	return "OPENAI_API_KEY"
}

func (r *SBX) allowGatewayNetwork(ctx context.Context, sandbox string, target testcase.Target) error {
	endpoint, err := url.Parse(target.BaseURL)
	if err != nil || endpoint.Hostname() == "" {
		return fmt.Errorf("parse gateway host for sandbox network policy")
	}
	output, err := exec.CommandContext(ctx, r.Executable, "policy", "allow", "network", "--sandbox", sandbox, endpoint.Hostname()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("allow scoped sbx gateway network: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func agentCommand(request testcase.AgentRequest, target testcase.Target) []string {
	if request.Agent == "codex" {
		return []string{
			"codex", "exec", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox",
			"--model", target.Model,
			"--config", `model_provider="agtk"`,
			"--config", `model_providers.agtk.name="AI Gateway Testkit"`,
			"--config", "model_providers.agtk.base_url=" + strconv.Quote(target.BaseURL),
			"--config", `model_providers.agtk.env_key="OPENAI_API_KEY"`,
			"--config", `model_providers.agtk.wire_api="responses"`,
			request.Prompt,
		}
	}
	return []string{"claude", "--print", "--no-session-persistence", "--dangerously-skip-permissions", "--model", target.Model, request.Prompt}
}

func randomSuffix() string {
	data := make([]byte, 4)
	if _, err := rand.Read(data); err != nil {
		return "run"
	}
	return hex.EncodeToString(data)
}

func lastNonEmptyLine(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return ""
}
