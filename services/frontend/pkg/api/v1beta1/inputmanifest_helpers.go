package v1beta1

const (
	SEPARATOR = "-"

	// Claudie cluster statuses
	STATUS_NEW         = "NEW"
	STATUS_DONE        = "DONE"
	STATUS_ERROR       = "ERROR"
	STATUS_IN_PROGRESS = "IN_PROGRESS"

	STAGE_BEGINING = "NONE"
)

// GetNamespacedName returns a string in Namespace/Name format
func (im *InputManifest) GetNamespacedName() string {
	return im.Namespace + SEPARATOR + im.Name
}

func (im *InputManifest) GetStatuses() InputManifestStatus {
	return im.Status
}

func (im *InputManifest) SetNewReousrceStatus() {
	im.Status.State = STATUS_NEW
	im.Status.Phase = STAGE_BEGINING
	im.Status.Message = "Schedguled for creation"
}

func (im *InputManifest) SetUpdateResourceStatus(newStatus InputManifestStatus) {
	im.Status = newStatus
}

func (im *InputManifest) SetDeletingStatus() {
	im.Status.State = STATUS_IN_PROGRESS
	im.Status.Phase = STAGE_BEGINING
	im.Status.Message = "Schedguled for deletion"
}

