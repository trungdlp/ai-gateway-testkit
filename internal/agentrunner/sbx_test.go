package agentrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestFixtureDoesNotContainCredential(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	target := testcase.Target{Model: "test-model", BaseURL: "https://gateway.example/v1", Credential: "top-secret"}
	require.NoError(t, writeFixture(workspace, testcase.AgentRequest{Protocol: "openai"}, target))

	entries, err := os.ReadDir(workspace)
	require.NoError(t, err)
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(workspace, entry.Name()))
		require.NoError(t, err)
		assert.NotContains(t, string(content), target.Credential)
	}
}

func TestAgentExecArgumentsUseAnthropicGatewayRootWithoutEnvFileOrCredential(t *testing.T) {
	t.Parallel()
	target := testcase.Target{Model: "test-model", BaseURL: "https://gateway.example/anthropic/v1", Credential: "top-secret"}
	args := agentExecArguments(testcase.AgentRequest{Agent: "claude", Protocol: "anthropic"}, target, "placeholder", "/workspace", "sandbox")
	joined := strings.Join(args, "\n")
	assert.NotContains(t, args, "--env-file")
	assert.Contains(t, joined, "ANTHROPIC_BASE_URL=https://gateway.example/anthropic")
	assert.Contains(t, joined, "ANTHROPIC_AUTH_TOKEN=placeholder")
	assert.NotContains(t, joined, "top-secret")
	assert.Contains(t, joined, "ANTHROPIC_API_KEY=")
	assert.False(t, strings.Contains(joined, "OPENAI_API_KEY"))
}

func TestAgentExecArgumentsPreserveUnversionedAnthropicBaseURL(t *testing.T) {
	t.Parallel()
	target := testcase.Target{Model: "test-model", BaseURL: "https://gateway.example/anthropic"}
	args := agentExecArguments(testcase.AgentRequest{Agent: "claude", Protocol: "anthropic"}, target, "placeholder", "/workspace", "sandbox")
	assert.Contains(t, strings.Join(args, "\n"), "ANTHROPIC_BASE_URL=https://gateway.example/anthropic")
}

func TestCodexCommandSelectsCustomResponsesProvider(t *testing.T) {
	t.Parallel()
	command := strings.Join(agentCommand(testcase.AgentRequest{Agent: "codex", Prompt: "task"}, testcase.Target{Model: "model", BaseURL: "https://gateway.example/v1"}), " ")
	assert.Contains(t, command, `model_provider="agtk"`)
	assert.Contains(t, command, `model_providers.agtk.base_url="https://gateway.example/v1"`)
	assert.Contains(t, command, `model_providers.agtk.wire_api="responses"`)
	assert.Contains(t, command, "--ephemeral")
}

func TestLastNonEmptyLineIgnoresSandboxStartupMessage(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/workspace", lastNonEmptyLine("Sandbox started successfully\n/workspace\n"))
}

func TestClaudeUsesBearerTokenEnvironment(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ANTHROPIC_AUTH_TOKEN", credentialEnvironment(testcase.AgentRequest{Agent: "claude", Protocol: "anthropic"}))
	assert.Equal(t, "OPENAI_API_KEY", credentialEnvironment(testcase.AgentRequest{Agent: "codex", Protocol: "openai"}))
}
