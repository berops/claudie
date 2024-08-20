package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"

	"gopkg.in/yaml.v3"

	"google.golang.org/protobuf/proto"

	"golang.org/x/crypto/ssh"
)

func createDesiredState(pending *store.Config) error {
	// 1. If the infrastructure was marked for deletion the InputManifest and the ManifestChecksum will be empty.
	markedForDeletion := pending.Manifest.Raw == "" &&
		pending.Manifest.Checksum == nil &&
		pending.Manifest.State == manifest.Pending.String()

	if markedForDeletion {
		for cluster := range pending.Clusters {
			pending.Clusters[cluster].Desired.K8s = nil
			pending.Clusters[cluster].Desired.LoadBalancers = nil
		}
		return nil
	}

	// In the next steps It might be the case either the Current State or Desired state is nil and thus
	// these cases needs to be handled gracefully.

	// 2. create the desired state from the input manifest.
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(pending.Manifest.Raw), &m); err != nil {
		return fmt.Errorf("error unmarshalling manifest for config %q: %w", pending.Name, err)
	}
	if err := createK8sClustersFromManifest(&m, pending); err != nil {
		return fmt.Errorf("failed to parse k8s clusters from manifest %q: %w", m.Name, err)
	}
	if err := createLBClustersFromManifest(&m, pending); err != nil {
		return fmt.Errorf("failed to parse lb clusters from manifest: %q: %w", m.Name, err)
	}

	// 2.1 Also consider to re-use existing data created in previous run if not first-run of the workflow for the manifest.
	if err := transferExistingState(&m, pending); err != nil {
		return fmt.Errorf("failed to reuse current state date for desired state for %q: %w", m.Name, err)
	}

	return nil
}

// createK8sClustersFromManifest parses manifest and creates desired state for Kubernetes clusters.
func createK8sClustersFromManifest(from *manifest.Manifest, into *store.Config) error {
	// 1. traverse all clusters in the manifest
	//    catching newly created or existing (updated).
	desiredClusters := make(map[string]struct{})
	for _, cluster := range from.Kubernetes.Clusters {
		newCluster := &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
				Name: strings.ToLower(cluster.Name),
				Hash: utils.CreateHash(utils.HashLength),
			},
			Kubernetes: cluster.Version,
			Network:    cluster.Network,
		}

		controlNodePools, err := from.CreateNodepools(cluster.Pools.Control, true)
		if err != nil {
			return fmt.Errorf("error while creating control nodepool for %s : %w", cluster.Name, err)
		}

		computeNodePools, err := from.CreateNodepools(cluster.Pools.Compute, false)
		if err != nil {
			return fmt.Errorf("error while creating compute nodepool for %s : %w", cluster.Name, err)
		}

		newCluster.ClusterInfo.NodePools = append(controlNodePools, computeNodePools...)

		if err := generateSSHKeys(newCluster.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", newCluster.ClusterInfo.Name, err)
		}

		clusterBytes, err := proto.Marshal(newCluster)
		if err != nil {
			return fmt.Errorf("failed to marshal k8s cluster %q: %w", utils.GetClusterID(newCluster.GetClusterInfo()), err)
		}

		// It is guaranteed by validation, that within a single InputManifest no two clusters (including LB)
		// can share the same name.
		if v, ok := into.Clusters[newCluster.ClusterInfo.Name]; ok {
			v.Desired.K8s = clusterBytes
		} else {
			if into.Clusters == nil {
				into.Clusters = make(map[string]*store.ClusterState)
			}
			into.Clusters[newCluster.ClusterInfo.Name] = &store.ClusterState{Desired: store.Clusters{K8s: clusterBytes}}
		}

		desiredClusters[newCluster.ClusterInfo.Name] = struct{}{}
	}

	// 2. Catch clusters that were deleted.
	for clusterName := range into.Clusters {
		if _, ok := desiredClusters[clusterName]; ok {
			continue
		}

		// Mark for deletion.
		into.Clusters[clusterName].Desired.K8s = nil
		into.Clusters[clusterName].Desired.LoadBalancers = nil
	}

	return nil
}

// createLBClustersFromManifest reads the manifest and creates load balancer clusters based on it.
func createLBClustersFromManifest(from *manifest.Manifest, into *store.Config) error {
	const hostnameHashLength = 17
	// 1. Collect all Lbs in the desired state for given K8s clusters.
	lbs := make(map[string]*spec.LoadBalancers)
	for _, lbCluster := range from.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, from)
		if err != nil {
			return fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}

		attachedRoles := getRolesAttachedToLBCluster(from.LoadBalancer.Roles, lbCluster.Roles)

		newLbCluster := &spec.LBcluster{
			ClusterInfo: &spec.ClusterInfo{
				Name: lbCluster.Name,
				Hash: utils.CreateHash(utils.HashLength),
			},
			Roles:       attachedRoles,
			Dns:         dns,
			TargetedK8S: lbCluster.TargetedK8s,
		}

		nodes, err := from.CreateNodepools(lbCluster.Pools, false)
		if err != nil {
			return fmt.Errorf("error while creating nodepools for %s : %w", lbCluster.Name, err)
		}
		newLbCluster.ClusterInfo.NodePools = nodes

		if err := generateSSHKeys(newLbCluster.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", newLbCluster.ClusterInfo.Name, err)
		}

		if newLbCluster.Dns.Hostname == "" {
			newLbCluster.Dns.Hostname = utils.CreateHash(hostnameHashLength)
		}

		if _, ok := lbs[newLbCluster.TargetedK8S]; !ok {
			lbs[newLbCluster.TargetedK8S] = new(spec.LoadBalancers)
		}
		lbs[newLbCluster.TargetedK8S].Clusters = append(lbs[newLbCluster.TargetedK8S].Clusters, newLbCluster)
	}

	// 2. Marshal and match with respective clusters.
	for k8sCluster := range lbs {
		state, ok := into.Clusters[k8sCluster]
		if !ok {
			// THIS  CASE SHOULD NEVER HAPPEN A LB CLUSTER CANNOT STAND ALONE WITHOUT A K8S CLUSTER.
			return fmt.Errorf("unexpected state. Loadbalancer cluster was created without a matching K8s cluster")
		}

		bytes, err := proto.Marshal(lbs[k8sCluster])
		if err != nil {
			return fmt.Errorf("failed to marshal lb clusters for %q: %w", k8sCluster, err)
		}

		state.Desired.LoadBalancers = bytes
	}

	return nil
}

// getDNS parses the manifest for the DNS specification.
func getDNS(dns manifest.DNS, from *manifest.Manifest) (*spec.DNS, error) {
	if dns.DNSZone == "" {
		return nil, fmt.Errorf("DNS zone not provided in manifest %s", from.Name)
	}

	provider, err := from.GetProvider(dns.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %s was not found in manifest %s", dns.Provider, from.Name)
	}

	return &spec.DNS{DnsZone: dns.DNSZone, Provider: provider, Hostname: dns.Hostname}, nil
}

// getRolesAttachedToLBCluster will read roles attached to the LB cluster from the unmarshalled manifest and return them.
// Returns slice of *[]pb.Roles if successful, error if Target from manifest state not found
func getRolesAttachedToLBCluster(roles []manifest.Role, roleNames []string) []*spec.Role {
	var matchingRoles []*spec.Role

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {
				var roleType spec.RoleType

				// The manifest validation is handling the check if the target nodepools of the
				// role are control nodepools and thus can be used as a valid API loadbalancer.
				// Given this invariant we can simply check for the port.
				if role.TargetPort == manifest.APIServerPort {
					roleType = spec.RoleType_ApiServer
				} else {
					roleType = spec.RoleType_Ingress
				}

				newRole := &spec.Role{
					Name:        role.Name,
					Protocol:    role.Protocol,
					Port:        role.Port,
					TargetPort:  role.TargetPort,
					TargetPools: role.TargetPools,
					RoleType:    roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}

	return matchingRoles
}

// transferExistingState transfers existing data from current state to desired.
func transferExistingState(m *manifest.Manifest, db *store.Config) error {
	// Since we're working with cluster states directly, we'll use the grpc form.
	// As in the DB form they're encoded.
	grpcRepr, err := store.ConvertToGRPC(db)
	if err != nil {
		return fmt.Errorf("failed to convert from db representation to grpc %q: %w", db.Name, err)
	}

	fixUpDuplicates(m, grpcRepr)

	if err := transferExistingK8sState(grpcRepr); err != nil {
		return fmt.Errorf("error while updating Kubernetes clusters for config %s : %w", db.Name, err)
	}

	if err := transferExistingLBState(grpcRepr); err != nil {
		return fmt.Errorf("error while updating Loadbalancer clusters for config %s : %w", db.Name, err)
	}

	newdb, err := store.ConvertFromGRPC(grpcRepr)
	if err != nil {
		return fmt.Errorf("failed to convert from grpc to db representation %q: %w", grpcRepr.Name, err)
	}

	*db = *newdb

	return nil
}

// fixUpDuplicates renames the nodepools if they're referenced multiple times in k8s,lb clusters.
func fixUpDuplicates(from *manifest.Manifest, config *spec.Config) {
	for _, state := range config.GetClusters() {
		desired := state.GetDesired().GetK8S()
		desiredLbs := state.GetDesired().GetLoadBalancers().GetClusters()

		current := state.GetCurrent().GetK8S()
		currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()

		for _, np := range from.NodePools.Dynamic {
			used := make(map[string]struct{})

			copyK8sNodePoolsNamesFromCurrentState(used, np.Name, current, desired)
			copyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

			references := findNodePoolReferences(np.Name, desired.GetClusterInfo().GetNodePools())
			for _, lb := range desiredLbs {
				references = append(references, findNodePoolReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
			}

			for _, np := range references {
				hash := utils.CreateHash(utils.HashLength)
				for _, ok := used[hash]; ok; {
					hash = utils.CreateHash(utils.HashLength)
				}
				used[hash] = struct{}{}
				np.Name += fmt.Sprintf("-%s", hash)
			}
		}
	}
}

// copyLbNodePoolNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBcluster) {
	for _, desired := range desired {
		references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
		switch {
		case len(references) > 1:
			panic("unexpected nodepool reference count")
		case len(references) == 0:
			continue
		}

		ref := references[0]

		for _, current := range current {
			if desired.ClusterInfo.Name != current.ClusterInfo.Name {
				continue
			}

			for _, np := range current.GetClusterInfo().GetNodePools() {
				_, hash := utils.GetNameAndHashFromNodepool(nodepool, np.Name)
				if hash == "" {
					continue
				}

				used[hash] = struct{}{}

				ref.Name += fmt.Sprintf("-%s", hash)
				break
			}
		}
	}
}

// copyK8sNodePoolsNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyK8sNodePoolsNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired *spec.K8Scluster) {
	references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
	switch {
	case len(references) == 0:
		return
	case len(references) > 2:
		panic("unexpected nodepool reference count")
	}

	// to avoid extra code for special cases where there is just 1 reference, append a nil.
	references = append(references, []*spec.NodePool{nil}...)

	control, compute := references[0], references[1]
	if !references[0].IsControl {
		control, compute = compute, control
	}

	for _, np := range current.GetClusterInfo().GetNodePools() {
		_, hash := utils.GetNameAndHashFromNodepool(nodepool, np.Name)
		if hash == "" {
			continue
		}

		used[hash] = struct{}{}

		if np.IsControl && control != nil {
			control.Name += fmt.Sprintf("-%s", hash)
		} else if !np.IsControl && compute != nil {
			compute.Name += fmt.Sprintf("-%s", hash)
		}
	}
}

// findNodePoolReferences find all nodepools that share the given name.
func findNodePoolReferences(name string, nodePools []*spec.NodePool) []*spec.NodePool {
	var references []*spec.NodePool
	for _, np := range nodePools {
		if np.Name == name {
			references = append(references, np)
		}
	}
	return references
}

// transferExistingK8sState updates the desired state of the kubernetes clusters based on the current state
func transferExistingK8sState(newConfig *spec.Config) error {
	for _, state := range newConfig.Clusters {
		if state.Desired == nil || state.Current == nil {
			continue
		}

		if err := updateClusterInfo(state.Desired.K8S.ClusterInfo, state.Current.K8S.ClusterInfo); err != nil {
			return err
		}

		// create SSH keys for new nodepools that were added.
		if err := generateSSHKeys(state.Desired.K8S.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", state.Desired.K8S.ClusterInfo.Name, err)
		}

		if state.Current.K8S.Kubeconfig != "" {
			state.Desired.K8S.Kubeconfig = state.Current.K8S.Kubeconfig
		}
	}
	return nil
}

// updateClusterInfo updates the desired state based on the current state
// namely:
// - Hash
// - AutoscalerConfig
// - existing nodes
// - nodepool
//   - metadata
//   - Public key
//   - Private key
func updateClusterInfo(desired, current *spec.ClusterInfo) error {
	desired.Hash = current.Hash
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			if desiredNp.Name != currentNp.Name {
				continue
			}

			switch {
			case tryUpdateDynamicNodePool(desiredNp, currentNp):
			case tryUpdateStaticNodePool(desiredNp, currentNp):
			default:
				return fmt.Errorf("%q is neither dynamic nor static, unexpected value: %v", desiredNp.Name, desiredNp.GetNodePoolType())
			}

			continue desired
		}
	}
	return nil
}

func tryUpdateDynamicNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetDynamicNodePool()
	cnp := current.GetDynamicNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey

	desired.Nodes = current.Nodes
	dnp.Cidr = cnp.Cidr

	// Update the count
	if cnp.AutoscalerConfig != nil && dnp.AutoscalerConfig != nil {
		// Both have Autoscaler conf defined, use same count as in current
		dnp.Count = cnp.Count
	} else if cnp.AutoscalerConfig == nil && dnp.AutoscalerConfig != nil {
		// Desired is autoscaled, but not current
		if dnp.AutoscalerConfig.Min > cnp.Count {
			// Cannot have fewer nodes than defined min
			dnp.Count = dnp.AutoscalerConfig.Min
		} else if dnp.AutoscalerConfig.Max < cnp.Count {
			// Cannot have more nodes than defined max
			dnp.Count = dnp.AutoscalerConfig.Max
		} else {
			// Use same count as in current for now, autoscaler might change it later
			dnp.Count = cnp.Count
		}
	}

	return true
}

func tryUpdateStaticNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetStaticNodePool()
	cnp := current.GetStaticNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	for _, dn := range desired.Nodes {
		for _, cn := range current.Nodes {
			if dn.Public == cn.Public {
				dn.Name = cn.Name
				dn.Private = cn.Private
				dn.NodeType = cn.NodeType
			}
		}
	}

	return true
}

// generateSSHKeys will generate SSH keypair for each nodepool that does not yet have
// a keypair assigned.
func generateSSHKeys(desiredInfo *spec.ClusterInfo) error {
	for i := range desiredInfo.NodePools {
		if dp := desiredInfo.NodePools[i].GetDynamicNodePool(); dp != nil && dp.PublicKey == "" {
			var err error
			if dp.PublicKey, dp.PrivateKey, err = generateSSHKeyPair(); err != nil {
				return fmt.Errorf("error while create SSH key pair for nodepool %s: %w", desiredInfo.NodePools[i].Name, err)
			}
		}
	}
	return nil
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// Generate and write public key
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pubKey))

	return pubKeyBuf.String(), privKeyBuf.String(), nil
}

// transferExistingLBState updates the desired state of the loadbalancer clusters based on the current state
func transferExistingLBState(newConfig *spec.Config) error {
	for _, state := range newConfig.Clusters {
		if state.Current == nil || state.Desired == nil {
			continue
		}

		currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()
		desiredLbs := state.GetDesired().GetLoadBalancers().GetClusters()

		for _, desired := range desiredLbs {
			for _, current := range currentLbs {
				if current.ClusterInfo.Name == desired.ClusterInfo.Name {
					if err := updateClusterInfo(desired.ClusterInfo, current.ClusterInfo); err != nil {
						return err
					}

					// create SSH keys for new nodepools that were added.
					if err := generateSSHKeys(desired.ClusterInfo); err != nil {
						return fmt.Errorf("error encountered while creating desired state for %s : %w", desired.GetClusterInfo().GetName(), err)
					}

					// copy hostname from current state if not specified in manifest
					// TODO: verify this is not needed.
					//if desired.Dns.Hostname == "" {
					//	desired.Dns.Hostname = current.Dns.Hostname
					//	desired.Dns.Endpoint = current.Dns.Endpoint
					//}
					if current.Dns.Hostname != desired.Dns.Hostname {
						desired.Dns.Endpoint = current.Dns.Endpoint
					}
					break
				}
			}
		}
	}

	return nil
}
