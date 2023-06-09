package v1beta1

import "fmt"

const (
	SEPARATOR = "/"
	// Claudie cluster statuses
	// IN_PROGRESS is a helper status thatindicates that the cluster is currently being build.
	STATUS_IN_PROGRESS = "IN_PROGRESS"
	// DONE_WITH_ERROR is a helper status that indicates that 
	// one of the clusters failed while building and the rest compleated succesfuly
	STATUS_DONE_ERROR = "DONE_WITH_ERROR"
	// ERROR indicates that an error occurred while building the cluster.
	STATUS_ERROR = "ERROR"
	// DONE indicates that the workflow has finished.
	STATUS_DONE = "DONE"
	// STATUS_NEW is a helper status that indicates that the resource was recently created
	STATUS_NEW  = "NEW"
	// SCHEDULED_FOR_DELETION
	STATUS_SCHEDULED_FOR_DELETION ="SCHEDULED_FOR_DELETION"

)

// GetNamespacedName returns a string in Namespace/Name format
func (im *InputManifest) GetNamespacedName() string {
	return im.Namespace + SEPARATOR + im.Name
}

// GetSecretField takes an ENUM string type of SecretField, and returns the value
// of the that field from the ProviderWithData struct
func (pwd *ProviderWithData) GetSecretField(name SecretField) (string, error) {
	if value, ok := pwd.Secret.Data[string(name)]; ok {
		return string(value), nil
	} else {
		return "", fmt.Errorf("field %s not found", name)
	}
}

// GetStatuses returns the inputmanifest.Status field
func (im *InputManifest) GetStatuses() InputManifestStatus {
	return im.Status
}

func (im *InputManifest) SetNewReousrceStatus() {
	im.Status.State = STATUS_NEW
}

func (im *InputManifest) SetUpdateResourceStatus(newStatus InputManifestStatus) {
	im.Status = newStatus
}

func (im *InputManifest) SetDeletingStatus() {
	im.Status.State = STATUS_SCHEDULED_FOR_DELETION
}
