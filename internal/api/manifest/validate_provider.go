package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// semverString verifies if a string has the semver 2.0 pattern. Ref: https://semver.org/
	semverString = `^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	semverRegex  = regexp.MustCompile(semverString)
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

	for _, c := range p.Openstack {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("failed to validate provider %q: %w", c.Name, err)
		}

		if _, ok := names[c.Name]; ok {
			return fmt.Errorf("name %q is used across multiple providers, must be unique", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range p.Exoscale {
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

func (c *GCP) Validate() error        { return validateProvider(c) }
func (c *Hetzner) Validate() error    { return validateProvider(c) }
func (c *OCI) Validate() error        { return validateProvider(c) }
func (c *Azure) Validate() error      { return validateProvider(c) }
func (c *AWS) Validate() error        { return validateProvider(c) }
func (c *Cloudflare) Validate() error { return validateProvider(c) }
func (c *Openstack) Validate() error  { return validateProvider(c) }
func (c *Exoscale) Validate() error   { return validateProvider(c) }

func validateSemver2(fl validator.FieldLevel) bool {
	semverString := fl.Field().String()
	// drop the 'v' as it's not part of a semantic version (https://semver.org/)
	semverString = strings.TrimPrefix(semverString, "v")
	return semverRegex.MatchString(semverString)
}

func validateProvider(provider any) error {
	validate := validator.New()

	if err := validate.RegisterValidation("semver2", validateSemver2, false); err != nil {
		return err
	}

	return prettyPrintValidationError(validate.Struct(provider))
}
