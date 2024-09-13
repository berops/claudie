package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"strings"
)

// Validate validates the parsed manifest data.
func (m *Manifest) Validate() error {
	if err := validator.New().Struct(m); err != nil {
		return fmt.Errorf("failed to validate manifest: %w", err)
	}

	// Check if at least one provider is defined
	// https://github.com/berops/claudie/blob/master/docs/input-manifest/input-manifest.md#providers
	providers := len(m.Providers.GCP) + len(m.Providers.Hetzner) + len(m.Providers.AWS) +
		len(m.Providers.Azure) + len(m.Providers.OCI) + len(m.Providers.Cloudflare) +
		len(m.Providers.GenesisCloud) + len(m.Providers.HetznerDNS)
	if providers < 1 {
		// Return error only if at least one dynamic nodepool defined.
		if len(m.NodePools.Dynamic) > 0 {
			return fmt.Errorf("need to define at least one provider inside the providers section of the manifest used for dynamic nodepools")
		}
	}

	if err := m.NodePools.Validate(m); err != nil {
		return fmt.Errorf("failed to validate nodepools section inside manifest: %w", err)
	}

	if err := m.Kubernetes.Validate(m); err != nil {
		return fmt.Errorf("failed to validate kubernetes section inside manifest: %w", err)
	}

	if err := m.LoadBalancer.Validate(m); err != nil {
		return fmt.Errorf("failed to validate loadbalancers section inside manifest: %w", err)
	}

	if err := CheckLengthOfFutureDomain(m); err != nil {
		return fmt.Errorf("failed to validate future domains: %w", err)
	}

	return nil
}

type validationErrors []error

func (v validationErrors) Error() string {
	buffer := bytes.NewBufferString("")

	for _, e := range v {
		buffer.WriteString(e.Error())
		buffer.WriteString("; ")
	}

	return strings.TrimSpace(buffer.String())
}

func prettyPrintValidationError(err error) error {
	var validationErr validator.ValidationErrors
	errors.As(err, &validationErr)

	if validationErr == nil {
		return err
	}

	var out validationErrors
	for _, err := range validationErr {
		var nerr error
		switch err.Tag() {
		case "max":
			nerr = fmt.Errorf("field '%s' must have a maximum length of %s", err.StructField(), err.Param())
		case "min":
			nerr = fmt.Errorf("field '%s' must have a minimum length of %s", err.StructField(), err.Param())
		case "len":
			nerr = fmt.Errorf("field '%s' must have exact length of %s", err.StructField(), err.Param())
		case "required":
			nerr = fmt.Errorf("field '%s' is required to be defined", err.StructField())
		case "ip_addr":
			nerr = fmt.Errorf("field '%s' is required to have a valid IPv4 address value", err.StructField())
		case "cidrv4":
			nerr = fmt.Errorf("field '%s' is required to have a valid CIDRv4 value", err.StructField())
		case "ver":
			nerr = fmt.Errorf("field '%s' is required to have a kubernetes version of: 1.27.x, 1.28.x, 1.29.x, 1.30.x", err.StructField())
		case "semver2":
			nerr = fmt.Errorf("field '%s' is required to follow semantic version 2.0, ref: https://semver.org/", err.StructField())
		default:
			nerr = err
		}
		out = append(out, nerr)
	}
	return out
}
