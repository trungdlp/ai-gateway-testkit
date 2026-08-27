package profile

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/trungdlp/ai-gateway-testkit/internal/catalog"
)

//go:embed definitions/*.yaml
var files embed.FS

type Definition struct {
	ID          string   `json:"id" yaml:"id"`
	Version     string   `json:"version" yaml:"version"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Includes    []string `json:"includes,omitempty" yaml:"includes"`
	Required    []string `json:"required,omitempty" yaml:"required"`
	Optional    []string `json:"optional,omitempty" yaml:"optional"`
}

type Registry struct {
	byID map[string]Definition
}

func Load(c *catalog.Catalog) (*Registry, error) {
	entries, err := files.ReadDir("definitions")
	if err != nil {
		return nil, fmt.Errorf("read embedded profiles: %w", err)
	}
	r := &Registry{byID: make(map[string]Definition, len(entries))}
	for _, entry := range entries {
		data, err := files.ReadFile("definitions/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var definition Definition
		if err := yaml.Unmarshal(data, &definition); err != nil {
			return nil, fmt.Errorf("parse profile %s: %w", entry.Name(), err)
		}
		if definition.ID == "" || definition.Version == "" || definition.Title == "" {
			return nil, fmt.Errorf("profile %s has incomplete metadata", entry.Name())
		}
		if _, exists := r.byID[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate profile ID %s", definition.ID)
		}
		r.byID[definition.ID] = definition
	}
	for _, definition := range r.byID {
		if _, err := r.Expand(definition.ID, c); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Get(id string) (Definition, bool) {
	definition, ok := r.byID[id]
	return definition, ok
}

func (r *Registry) Definitions() []Definition {
	result := make([]Definition, 0, len(r.byID))
	for _, definition := range r.byID {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

type Selection struct {
	Required map[string]struct{}
	Optional map[string]struct{}
}

func (r *Registry) Expand(id string, c *catalog.Catalog) (Selection, error) {
	selection := Selection{Required: map[string]struct{}{}, Optional: map[string]struct{}{}}
	visiting := map[string]bool{}
	var expand func(string) error
	expand = func(profileID string) error {
		if visiting[profileID] {
			return fmt.Errorf("profile include cycle at %s", profileID)
		}
		definition, ok := r.byID[profileID]
		if !ok {
			return fmt.Errorf("unknown profile %s", profileID)
		}
		visiting[profileID] = true
		defer delete(visiting, profileID)
		for _, included := range definition.Includes {
			if err := expand(included); err != nil {
				return err
			}
		}
		for _, reference := range definition.Required {
			ids, err := expandReference(reference, c)
			if err != nil {
				return fmt.Errorf("profile %s: %w", profileID, err)
			}
			for _, assertionID := range ids {
				selection.Required[assertionID] = struct{}{}
				delete(selection.Optional, assertionID)
			}
		}
		for _, reference := range definition.Optional {
			ids, err := expandReference(reference, c)
			if err != nil {
				return fmt.Errorf("profile %s: %w", profileID, err)
			}
			for _, assertionID := range ids {
				if _, required := selection.Required[assertionID]; !required {
					selection.Optional[assertionID] = struct{}{}
				}
			}
		}
		return nil
	}
	return selection, expand(id)
}

func expandReference(reference string, c *catalog.Catalog) ([]string, error) {
	if strings.Contains(reference, "/") {
		if _, ok := c.Assertion(reference); !ok {
			return nil, fmt.Errorf("unknown assertion %s", reference)
		}
		return []string{reference}, nil
	}
	definition, ok := c.Get(reference)
	if !ok {
		return nil, fmt.Errorf("unknown case %s", reference)
	}
	result := make([]string, 0, len(definition.Assertions))
	for _, assertion := range definition.Assertions {
		result = append(result, definition.ID+"/"+assertion.ID)
	}
	return result, nil
}
