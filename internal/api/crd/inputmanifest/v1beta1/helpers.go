package v1beta1

import (
	"fmt"
	"unicode/utf8"

	"github.com/berops/claudie/internal/api/manifest"
)

const (
	SEPARATOR = "/"
	// Claudie cluster statuses
	// IN_PROGRESS is a helper status that indicates that the cluster is currently being build.
	STATUS_IN_PROGRESS = "IN_PROGRESS"
	// ERROR indicates that an error occurred while building the cluster.
	STATUS_ERROR = "ERROR"
	// DONE indicates that the workflow has finished.
	STATUS_DONE = "DONE"
	// STATUS_NEW is a helper status that indicates that the resource was recently created
	STATUS_NEW = "NEW"
	// SCHEDULED_FOR_DELETION
	STATUS_SCHEDULED_FOR_DELETION = "SCHEDULED_FOR_DELETION"
)

// GetNamespacedName returns a string in Namespace/Name format
func (im *InputManifest) GetNamespacedName() string {
	if im.Namespace == "" {
		return "default" + SEPARATOR + im.Name
	}
	return im.Namespace + SEPARATOR + im.Name
}

// GetNamespacedNameDashed returns a string in Namespace-Name format
func (im *InputManifest) GetNamespacedNameDashed() string {
	if im.Namespace == "" {
		return "default" + "-" + im.Name
	}
	return im.Namespace + "-" + im.Name
}

// GetSecretField takes an ENUM string type of SecretField, and returns the value
// of the that field from the ProviderWithData struct
// it is also validating if the string is a proper UTF-8 string
func (pwd *ProviderWithData) GetSecretField(name SecretField) (string, error) {
	if value, ok := pwd.Secret.Data[string(name)]; ok {
		// Ref: https://github.com/berops/claudie/issues/1101#issuecomment-1820793262
		if !utf8.ValidString(string(value)) {
			return "", fmt.Errorf("field %s is not a valid UTF-8 string", name)
		}
		return string(value), nil
	} else {
		return "", fmt.Errorf("field %s not found", name)
	}
}

// GetStatuses returns the inputmanifest.Status field
func (im *InputManifest) GetStatuses() InputManifestStatus {
	return im.Status
}

func (im *InputManifest) SetNewResourceStatus() {
	im.Status.State = STATUS_NEW
}

func (im *InputManifest) SetUpdateResourceStatus(newStatus InputManifestStatus) {
	im.Status = newStatus
}

func (im *InputManifest) SetDeletingStatus() {
	im.Status.State = STATUS_SCHEDULED_FOR_DELETION
}

// Translates the CRD role to the Manifest Role.
func (r *Role) IntoManifestRole() manifest.Role {
	// This function is placed in this module to avoid import cycles.
	return manifest.Role{
		Name:        r.Name,
		Protocol:    r.Protocol,
		Port:        r.Port,
		TargetPort:  r.TargetPort,
		TargetPools: r.TargetPools,
		Settings:    r.Settings,
		// EnvoyProxy cannot be set as it is
		// fetch by the controller when the
		// InputManifest is created.
		EnvoyProxy: &manifest.EnvoyProxy{},
	}
}
