package manifest

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validate validates the parsed data inside the NodePool section of the manifest.
// It checks for missing/invalid filled out values defined in the NodePool section of
// the manifest.
func (p *NodePool) Validate(m *Manifest) error {
	for _, n := range p.Dynamic {
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate DynamicNodePool %q: %w", n.Name, err)
		}

		// check if the provider is defined in the manifest
		if _, err := m.GetProvider(n.ProviderSpec.Name); err != nil {
			return fmt.Errorf("provider %q specified for DynamicNodePool %q doesn't exists", n.ProviderSpec.Name, n.Name)
		}
	}

	for _, n := range p.Static {
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate StaticNodePool %q: %w", n.Name, err)
		}
	}

	return nil
}

func (d *DynamicNodePool) Validate() error { return validator.New().Struct(d) }
func (s *StaticNodePool) Validate() error  { return validator.New().Struct(s) }
