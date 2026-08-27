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

func TestWriteEnvironmentContainsNoCredential(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "env")
	require.NoError(t, err)
	target := testcase.Target{Model: "test-model", BaseURL: "https://gateway.example/v1", Credential: "top-secret"}
	require.NoError(t, writeEnvironment(file, testcase.AgentRequest{Protocol: "anthropic"}, target))
	require.NoError(t, file.Close())
	content, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	assert.Contains(t, string(content), "ANTHROPIC_BASE_URL=https://gateway.example/v1")
	assert.NotContains(t, string(content), "top-secret")
	assert.NotContains(t, string(content), "ANTHROPIC_API_KEY")
	assert.False(t, strings.Contains(string(content), "OPENAI_API_KEY"))
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
