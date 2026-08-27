package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func main() {
	check := flag.Bool("check", false, "fail when generated catalog files are stale")
	flag.Parse()
	catalog, err := cases.Catalog()
	if err != nil {
		fail(err)
	}
	profiles, err := profile.Load(catalog)
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(catalog.Document(), "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	markdown := renderMarkdown(catalog.Definitions(), profiles.Definitions())
	outputs := map[string][]byte{"catalog/catalog.json": data, "docs/test-catalog.md": []byte(markdown)}
	for path, content := range outputs {
		if *check {
			existing, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(existing, content) {
				fail(fmt.Errorf("generated file %s is stale; run go generate ./...", path))
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			fail(err)
		}
	}
}

func renderMarkdown(definitions []testcase.Definition, profiles []profile.Definition) string {
	var output strings.Builder
	output.WriteString("# Test catalog\n\nThis file is generated. Run `go generate ./...` after changing cases or profiles.\n\n")
	output.WriteString("## Scenarios\n\n| ID | Revision | Layer | Stability | Assertions | Title |\n| --- | ---: | --- | --- | ---: | --- |\n")
	for _, definition := range definitions {
		fmt.Fprintf(&output, "| `%s` | %d | %s | %s | %d | %s |\n", definition.ID, definition.Revision, definition.Layer, definition.Stability, len(definition.Assertions), definition.Title)
	}
	output.WriteString("\n## Profiles\n\n| ID | Version | Included profiles | Required references | Title |\n| --- | --- | --- | ---: | --- |\n")
	for _, definition := range profiles {
		fmt.Fprintf(&output, "| `%s` | %s | %s | %d | %s |\n", definition.ID, definition.Version, strings.Join(definition.Includes, ", "), len(definition.Required), definition.Title)
	}
	return output.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
