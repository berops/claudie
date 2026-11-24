package manifest

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// ValidateVersionUpgrade checks whether a cluster running the current kubernetes
// version can move to the desired version. Keeping the same version and patch
// upgrades are allowed, as is an upgrade to the next minor version. Downgrades,
// major version changes and upgrades that skip a minor version are rejected,
// as kubeadm only supports upgrading one minor version at a time.
func ValidateVersionUpgrade(current, desired string) error {
	cur, err := semver.NewVersion(current)
	if err != nil {
		return fmt.Errorf("invalid current kubernetes version %q: %w", current, err)
	}

	des, err := semver.NewVersion(desired)
	if err != nil {
		return fmt.Errorf("invalid desired kubernetes version %q: %w", desired, err)
	}

	if des.LessThan(cur) {
		return fmt.Errorf("downgrading kubernetes from %s to %s is not supported", cur, des)
	}

	if des.Major() != cur.Major() {
		return fmt.Errorf("changing the kubernetes major version from %s to %s is not supported", cur, des)
	}

	if des.Minor() > cur.Minor()+1 {
		return fmt.Errorf(
			"upgrading kubernetes from %s to %s skips a minor version, upgrade to 1.%d first",
			cur, des, cur.Minor()+1,
		)
	}

	return nil
}
