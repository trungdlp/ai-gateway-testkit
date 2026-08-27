package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncDirectoryWritesExpectedAndRemovesOrphans(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "ORPHAN.json"), []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "README.md"), []byte("keep"), 0o644))

	expected := map[string][]byte{"OAI-AUTH-001.json": []byte("new\n")}
	require.NoError(t, syncDirectory(directory, ".json", expected, false))

	data, err := os.ReadFile(filepath.Join(directory, "OAI-AUTH-001.json"))
	require.NoError(t, err)
	require.Equal(t, []byte("new\n"), data)
	require.NoFileExists(t, filepath.Join(directory, "ORPHAN.json"))
	require.FileExists(t, filepath.Join(directory, "README.md"))
	require.NoError(t, syncDirectory(directory, ".json", expected, true))
}

func TestSyncDirectoryCheckRejectsStaleAndOrphanFiles(t *testing.T) {
	directory := t.TempDir()
	expected := map[string][]byte{"OAI-AUTH-001.json": []byte("new\n")}
	require.NoError(t, os.WriteFile(filepath.Join(directory, "OAI-AUTH-001.json"), []byte("old\n"), 0o644))

	err := syncDirectory(directory, ".json", expected, true)
	require.ErrorContains(t, err, "is stale")

	require.NoError(t, os.WriteFile(filepath.Join(directory, "OAI-AUTH-001.json"), []byte("new\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "ORPHAN.json"), []byte("old\n"), 0o644))
	err = syncDirectory(directory, ".json", expected, true)
	require.ErrorContains(t, err, "orphan generated file")
}
