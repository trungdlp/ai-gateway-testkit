package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const Version = "2026.08.0"

var (
	caseIDPattern      = regexp.MustCompile(`^[A-Z]{2,4}-[A-Z]{3,5}-[0-9]{3}$`)
	assertionIDPattern = regexp.MustCompile(`^A[0-9]{2}$`)
)

type Catalog struct {
	definitions []testcase.Definition
	byID        map[string]testcase.Definition
}

type Document struct {
	Version string                `json:"version"`
	Cases   []testcase.Definition `json:"cases"`
}

func New(definitions []testcase.Definition) (*Catalog, error) {
	copyOf := append([]testcase.Definition(nil), definitions...)
	sort.Slice(copyOf, func(i, j int) bool { return copyOf[i].ID < copyOf[j].ID })
	c := &Catalog{definitions: copyOf, byID: make(map[string]testcase.Definition, len(copyOf))}
	for _, definition := range copyOf {
		if err := validateDefinition(definition); err != nil {
			return nil, err
		}
		if _, exists := c.byID[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate case ID %s", definition.ID)
		}
		c.byID[definition.ID] = definition
	}
	for _, definition := range copyOf {
		for _, dependency := range definition.DependsOn {
			if _, exists := c.byID[dependency]; !exists {
				return nil, fmt.Errorf("case %s depends on unknown case %s", definition.ID, dependency)
			}
		}
	}
	if err := c.validateAcyclic(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) Definitions() []testcase.Definition {
	return append([]testcase.Definition(nil), c.definitions...)
}

func (c *Catalog) Get(id string) (testcase.Definition, bool) {
	definition, ok := c.byID[id]
	return definition, ok
}

func (c *Catalog) Assertion(fullID string) (testcase.AssertionDefinition, bool) {
	parts := strings.Split(fullID, "/")
	if len(parts) != 2 {
		return testcase.AssertionDefinition{}, false
	}
	definition, ok := c.Get(parts[0])
	if !ok {
		return testcase.AssertionDefinition{}, false
	}
	for _, assertion := range definition.Assertions {
		if assertion.ID == parts[1] {
			return assertion, true
		}
	}
	return testcase.AssertionDefinition{}, false
}

func (c *Catalog) Document() Document {
	return Document{Version: Version, Cases: c.Definitions()}
}

func (c *Catalog) Digest() string {
	data, _ := json.Marshal(c.Document())
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validateDefinition(definition testcase.Definition) error {
	if !caseIDPattern.MatchString(definition.ID) {
		return fmt.Errorf("invalid case ID %q", definition.ID)
	}
	if definition.Revision < 1 || definition.Title == "" || definition.Description == "" {
		return fmt.Errorf("case %s requires revision, title, and description", definition.ID)
	}
	if definition.Layer == "" || definition.Suite == "" || definition.Area == "" || definition.Stability == "" || definition.Determinism == "" {
		return fmt.Errorf("case %s has incomplete classification metadata", definition.ID)
	}
	if !validLayer(definition.Layer) || !validStability(definition.Stability) || !validDeterminism(definition.Determinism) {
		return fmt.Errorf("case %s has invalid classification metadata", definition.ID)
	}
	if definition.Cost.Requests < 0 || definition.Cost.MaxInputTokens < 0 || definition.Cost.MaxOutputTokens < 0 {
		return fmt.Errorf("case %s has negative cost metadata", definition.ID)
	}
	if len(definition.Assertions) == 0 || definition.Run == nil {
		return fmt.Errorf("case %s requires assertions and a runner", definition.ID)
	}
	seen := make(map[string]struct{}, len(definition.Assertions))
	for _, assertion := range definition.Assertions {
		if !assertionIDPattern.MatchString(assertion.ID) || assertion.Title == "" || !validRequirement(assertion.Requirement) || !validImpact(assertion.Impact) {
			return fmt.Errorf("case %s has invalid assertion %q", definition.ID, assertion.ID)
		}
		if _, exists := seen[assertion.ID]; exists {
			return fmt.Errorf("case %s has duplicate assertion %s", definition.ID, assertion.ID)
		}
		seen[assertion.ID] = struct{}{}
	}
	return nil
}

func validLayer(value testcase.Layer) bool {
	switch value {
	case testcase.LayerProtocol, testcase.LayerSDK, testcase.LayerBehavioral, testcase.LayerAgent, testcase.LayerSecurity, testcase.LayerResilience:
		return true
	default:
		return false
	}
}

func validStability(value testcase.Stability) bool {
	switch value {
	case testcase.StabilityExperimental, testcase.StabilityStable, testcase.StabilityDeprecated, testcase.StabilityRetired:
		return true
	default:
		return false
	}
}

func validDeterminism(value testcase.Determinism) bool {
	switch value {
	case testcase.Deterministic, testcase.Probabilistic, testcase.Operational:
		return true
	default:
		return false
	}
}

func validRequirement(value testcase.Requirement) bool {
	return value == testcase.Must || value == testcase.Should || value == testcase.May
}

func validImpact(value testcase.Impact) bool {
	return value == testcase.Blocker || value == testcase.High || value == testcase.Medium || value == testcase.Low
}

func (c *Catalog) validateAcyclic() error {
	const (
		unseen = iota
		visiting
		done
	)
	state := make(map[string]int, len(c.byID))
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == visiting {
			return fmt.Errorf("dependency cycle includes %s", id)
		}
		if state[id] == done {
			return nil
		}
		state[id] = visiting
		for _, dependency := range c.byID[id].DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = done
		return nil
	}
	for id := range c.byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
