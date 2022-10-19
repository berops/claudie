package manifest

const (
	maxLength  = 80 // total length of domain = 8 + len(publicIP)[15] + 19 + len(NodeName) + margin
	baseLength = 8 + 19 + 15
)

// CheckLengthOfFutureDomain will check if the possible domain name is too long
// returns error if domain will be too long, nil if not
// Described in https://github.com/Berops/claudie/issues/112#issuecomment-1015432224
func checkLengthOfFutureDomain(m *Manifest) error {
	// https://<public-ip>:6443/<api-path>/<node-name>
	// <node-name> = clusterName + hash + nodeName + indexLength + separators

	return nil
}
