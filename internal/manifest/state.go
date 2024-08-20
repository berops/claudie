package manifest

import (
	"fmt"
	"slices"
	"strings"
)

// The State of the Manifest represent when given "actions" can be done.
// Each newly created Manifest starts in the Pending state. Pending Manifests
// will be picked up and based on the provided InputManifest specification desired
// state will be created for each cluster along with the tasks to be done to achieve
// the desired state from the current state, after which the Manifest will be moved to the
// scheduled state. Scheduled Manifests will be picked up and individual tasks will be worked on
// by other services. From this state the Manifest can end up in the Done or Error state. Any changes
// made to the Input Manifest while in the Scheduled state will be reflected after it has been move
// to the Done stage. So that the Read/Write/Update cycle repeats.
//
//go:generate stringer -type=State
type State int

const (
	// Pending state represents a new manifest that was created or an existing manifest
	// that was updated with new input.
	Pending State = iota
	// Scheduled state represents a manifest that was scheduled to be built.
	Scheduled
	// Done state represents all state was build successfully.
	Done
	// Error state represents an error occurred during the built of the manifest.
	Error
)

// StateTransitionMap describes which states are accessible from which.
var StateTransitionMap = map[State][]State{
	Pending:   {Pending, Scheduled},
	Scheduled: {Scheduled, Done, Error},
	Done:      {Done, Pending},
	Error:     {Error, Pending},
}

// ValidStateTransition validates if the state transition is acceptable.
func ValidStateTransition(src, dst State) bool { return slices.Contains(StateTransitionMap[src], dst) }

func ValidStateTransitionString(src string, dst State) (bool, error) {
	switch src {
	case "Pending":
		return ValidStateTransition(Pending, dst), nil
	case "Scheduled":
		return ValidStateTransition(Scheduled, dst), nil
	case "Done":
		return ValidStateTransition(Done, dst), nil
	case "Error":
		return ValidStateTransition(Error, dst), nil
	default:
		return false, fmt.Errorf("unrecognized state %q", src)
	}
}

func StateFromString(src string) (State, error) {
	switch strings.ToLower(src) {
	case "pending":
		return Pending, nil
	case "scheduled":
		return Scheduled, nil
	case "done":
		return Done, nil
	case "error":
		return Error, nil
	default:
		return Error, fmt.Errorf("unrecognized state %q", src)
	}
}
