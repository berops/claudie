package manifest

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validate validates the parsed data inside the provider section of the manifest.
// It checks for missing/invalid filled out values defined in the Provider section of
// the manifest.
func (p *Provider) Validate() error {
	// check for unique names across all cloud providers.
	names := make(map[string]bool)

	for _, c := range p.GCP {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}

		names[c.Name] = true
	}

	for _, c := range p.Hetzner {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}

		names[c.Name] = true
	}

	for _, c := range p.OCI {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range p.AWS {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range p.Azure {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range p.Cloudflare {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range p.HetznerDNS {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	return nil
}

func (c *GCP) Validate() error        { return validator.New().Struct(c) }
func (c *Hetzner) Validate() error    { return validator.New().Struct(c) }
func (c *OCI) Validate() error        { return validator.New().Struct(c) }
func (c *Azure) Validate() error      { return validator.New().Struct(c) }
func (c *AWS) Validate() error        { return validator.New().Struct(c) }
func (c *Cloudflare) Validate() error { return validator.New().Struct(c) }
func (c *HetznerDNS) Validate() error { return validator.New().Struct(c) }
