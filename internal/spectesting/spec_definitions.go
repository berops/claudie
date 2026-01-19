package spectesting

import (
	crand "crypto/rand"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"net/netip"
	"slices"
	"sync"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
)

type Rng struct {
	l sync.Mutex
	g *rand.ChaCha8
}

func (r *Rng) Read(b []byte) (n int, err error) {
	r.l.Lock()
	defer r.l.Unlock()
	return r.g.Read(b)
}

func (r *Rng) Uint64() uint64 {
	r.l.Lock()
	defer r.l.Unlock()
	return r.g.Uint64()
}

var (
	KnownProviderTypes map[string]any
	ProviderTypeOption map[string]func(any) FakeProviderOption

	PublicNetwork = must(netip.ParsePrefix("10.0.1.0/24"))

	rng = Rng{
		l: sync.Mutex{},
		g: rand.NewChaCha8([32]byte{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
		}),
	}
)

func init() {
	var seed [32]byte
	must(crand.Read(seed[:]))
	rng.g = rand.NewChaCha8(seed)

	KnownProviderTypes = make(map[string]any)
	ProviderTypeOption = make(map[string]func(any) FakeProviderOption)

	KnownProviderTypes["gcp"] = CreateFakeProviderGCP(
		WithProviderGCPKey("secret-key"),
		WithProviderGCPProject("project001"),
	)
	ProviderTypeOption["gcp"] = func(a any) FakeProviderOption {
		return WithProviderGCP(a.(*spec.Provider_Gcp))
	}
	KnownProviderTypes["hetzner"] = CreateFakeProviderHetzner(
		WithProviderHetznerToken("token"),
	)
	ProviderTypeOption["hetzner"] = func(a any) FakeProviderOption {
		return WithProviderHetzner(a.(*spec.Provider_Hetzner))
	}
	KnownProviderTypes["hetznerdns"] = CreateFakeProviderHetznerdns(
		WithProviderHetznerdnsToken("dnstoken"),
	)
	ProviderTypeOption["hetznerdns"] = func(a any) FakeProviderOption {
		return WithProviderHetznerdns(a.(*spec.Provider_Hetznerdns))
	}
	KnownProviderTypes["oci"] = CreateFakeProviderOci(
		WithProviderOciID("oci-id"),
		WithProviderOciTenancy("oci-tenancy"),
		WithProviderOciPrivateKey("private-key"),
		WithProviderOciCompartmentID("compartment-id"),
		WithProviderOciKeyFingerprint("fingerprint"),
	)
	ProviderTypeOption["oci"] = func(a any) FakeProviderOption {
		return WithProviderOci(a.(*spec.Provider_Oci))
	}
	KnownProviderTypes["aws"] = CreateFakeProviderAWS(
		WithProviderAWSSecretKey("secret-key"),
		WithProviderAWSAccessKey("access-key"),
	)
	ProviderTypeOption["aws"] = func(a any) FakeProviderOption {
		return WithProviderAws(a.(*spec.Provider_Aws))
	}
	KnownProviderTypes["azure"] = CreateFakeProviderAzure(
		WithProviderAzureSubId("subscription-id"),
		WithProviderAzureClientId("client-id"),
		WithProviderAzureTenantId("tenant-id"),
		WithProviderAzureClientSecret("client-secret"),
	)
	ProviderTypeOption["azure"] = func(a any) FakeProviderOption {
		return WithProviderAzure(a.(*spec.Provider_Azure))
	}
	KnownProviderTypes["cloudflare"] = CreateFakeProviderCloudflare(
		WithProviderCloudflareToken("token"),
	)
	ProviderTypeOption["cloudflare"] = func(a any) FakeProviderOption {
		return WithProviderCloudflare(a.(*spec.Provider_Cloudflare))
	}
}

func GenerateFakeProvider() *spec.Provider {
	r := make([]byte, 15)
	must(rng.Read(r))

	iter := int(uint32(rng.Uint64())) % len(KnownProviderTypes)
	var chosen string
	for p := range KnownProviderTypes {
		chosen = p
		iter--
		if iter == 0 {
			break
		}
	}

	return CreateFakeProvider(
		WithProviderSpecName(string(r)),
		WithProviderCloudProviderName(chosen),
		ProviderTypeOption[chosen](KnownProviderTypes[chosen]),
		WithProviderTemplates(
			CreateFakeRepository(
				WithRepository("127.0.0.1"),
				WithRepositoryTag("v0.0.1"),
				WithRepositoryPath("/tmp"),
				WithRepositoryCommitHash("0"),
			),
		),
	)
}

func GenerateFakeDNS() *spec.DNS {
	var (
		zone     = make([]byte, 20)
		hostname = make([]byte, 20)
		endpoint = make([]byte, 20)
	)

	must(rng.Read(zone))
	must(rng.Read(hostname))
	must(rng.Read(endpoint))

	return CreateFakeDNS(
		WithDNSZone(string(zone)),
		WithDNSHostname(string(hostname)),
		WithDNSProvider(GenerateFakeProvider()),
		WithDNSEndpoint(string(endpoint)),
	)
}

func GenerateFakeLBClusterInfo(
	privateNetworkCidr string,
	publicNetworkCidr string,
) *spec.ClusterInfo {
	var (
		name                = make([]byte, 50)
		hash                = make([]byte, 50)
		workerNodepools     = make([]*spec.NodePool, 0, 50)
		privateNetwork      = must(netip.ParsePrefix(privateNetworkCidr))
		privateStartAddress = privateNetwork.Addr()
		publicNetwork       = must(netip.ParsePrefix(publicNetworkCidr))
		publicStartAddress  = publicNetwork.Addr()
	)

	for range cap(workerNodepools) {
		var (
			nodeCount = max(15, rng.Uint64()%255)
			nodes     = make([]*spec.Node, 0, nodeCount)
			name      = make([]byte, 50)
		)

		must(rng.Read(name))

		for range cap(nodes) {
			var (
				name     = make([]byte, 50)
				username = make([]byte, 50)
			)
			must(rng.Read(name))
			must(rng.Read(username))

			if !privateNetwork.Contains(privateStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", privateStartAddress, privateNetwork))
			}
			if !publicNetwork.Contains(publicStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", publicStartAddress, publicNetwork))
			}

			nodes = append(nodes, CreateFakeNode(
				WithNodeName(string(name)),
				WithNodePrivate(privateStartAddress.String()),
				WithNodePublic(publicStartAddress.String()),
				WithNodeType(spec.NodeType_worker),
				WithNodeUsername(string(name)),
			))
			privateStartAddress = privateStartAddress.Next()
			publicStartAddress = publicStartAddress.Next()
		}

		if rng.Uint64()%2 == 0 { // static
			keys := make(map[string]string)
			for _, n := range nodes {
				key := make([]byte, 256)
				must(rng.Read(key))
				keys[n.Public] = string(key)
			}

			workerNodepools = append(workerNodepools, CreateFakeNodePool(
				WithNodePoolStaticType(CreateFakeStaticNodePool(WithStaticNodePoolNodeKeys(keys))),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		} else { // dynamic
			var (
				serverType = make([]byte, 50)
				image      = make([]byte, 50)
				region     = make([]byte, 50)
				zone       = make([]byte, 50)
				pk         = make([]byte, 50)
				sk         = make([]byte, 50)
				cidr       = make([]byte, 4)
				autoscaler *spec.AutoscalerConf
				count      = len(nodes)
			)

			must(rng.Read(serverType))
			must(rng.Read(image))
			must(rng.Read(region))
			must(rng.Read(zone))
			must(rng.Read(pk))
			must(rng.Read(sk))
			must(rng.Read(cidr[:3]))

			if rng.Uint64()%6 == 0 {
				minRange := 1 + (rng.Uint64() % nodeCount)
				maxRange := max(minRange, (rng.Uint64() % nodeCount))
				count = int(maxRange)
				nodes = nodes[:count]
				autoscaler = &spec.AutoscalerConf{
					Min: int32(minRange),
					Max: int32(maxRange),
				}
			}

			workerNodepools = append(workerNodepools, CreateFakeNodePool(
				WithNodePoolDynamicType(CreateFakeDynamicNodePool(
					WithDynamicNodePoolServerType(string(serverType)),
					WithDynamicNodePoolImage(string(image)),
					WithDynamicNodePoolStorageDiskSize(int32(rng.Uint64())),
					WithDynamicNodePoolRegion(string(region)),
					WithDynamicNodePoolZone(string(zone)),
					WithDynamicNodePoolCount(int32(count)),
					WithDynamicNodePoolProvider(GenerateFakeProvider()),
					WithDynamicNodePoolAutoscaler(autoscaler),
					WithDynamicNodePoolMachineSpec(nil),
					WithDynamicNodePoolPublicKey(string(pk)),
					WithDynamicNodePoolPrivateKey(string(sk)),
					WithDynamicNodePoolCIDR(string(cidr)+"/24"),
				)),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		}
	}

	must(rng.Read(name))
	must(rng.Read(hash))

	return CreateFakeClusterInfo(
		WithClusterInfoName(string(name)),
		WithClusterInfoHash(string(hash)),
		WithClusterInfoNodePools(workerNodepools),
	)
}

func GenerateFakeK8SClusterInfo(
	willHaveLbApiEndpoint bool,
	privateNetworkCidr string,
	publicNetworkCidr string,
) *spec.ClusterInfo {
	var (
		name                = make([]byte, 50)
		hash                = make([]byte, 50)
		controlNodepools    = make([]*spec.NodePool, 0, 50)
		workerNodepools     = make([]*spec.NodePool, 0, 50)
		privateNetwork      = must(netip.ParsePrefix(privateNetworkCidr))
		privateStartAddress = privateNetwork.Addr()
		publicNetwork       = must(netip.ParsePrefix(publicNetworkCidr))
		publicStartAddress  = publicNetwork.Addr()
	)

	for range cap(controlNodepools) {
		var (
			nodeCount = max(10, rng.Uint64()%255)
			nodes     = make([]*spec.Node, 0, nodeCount)
			name      = make([]byte, 50)
		)

		must(rng.Read(name))

		for range cap(nodes) {
			var (
				name     = make([]byte, 50)
				username = make([]byte, 50)
			)
			must(rng.Read(name))
			must(rng.Read(username))

			if !privateNetwork.Contains(privateStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", privateStartAddress, privateNetwork))
			}
			if !publicNetwork.Contains(publicStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", publicStartAddress, publicNetwork))
			}

			nodes = append(nodes, CreateFakeNode(
				WithNodeName(string(name)),
				WithNodePrivate(privateStartAddress.String()),
				WithNodePublic(publicStartAddress.String()),
				WithNodeType(spec.NodeType_master),
				WithNodeUsername(string(name)),
			))
			privateStartAddress = privateStartAddress.Next()
			publicStartAddress = publicStartAddress.Next()
		}

		if rng.Uint64()%2 == 0 { // static
			keys := make(map[string]string)
			for _, n := range nodes {
				key := make([]byte, 256)
				must(rng.Read(key))
				keys[n.Public] = string(key)
			}

			controlNodepools = append(controlNodepools, CreateFakeNodePool(
				WithNodePoolStaticType(CreateFakeStaticNodePool(WithStaticNodePoolNodeKeys(keys))),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		} else { // dynamic
			var (
				serverType = make([]byte, 50)
				image      = make([]byte, 50)
				region     = make([]byte, 50)
				zone       = make([]byte, 50)
				pk         = make([]byte, 50)
				sk         = make([]byte, 50)
				cidr       = make([]byte, 4)
				autoscaler *spec.AutoscalerConf
				count      = len(nodes)
			)

			must(rng.Read(serverType))
			must(rng.Read(image))
			must(rng.Read(region))
			must(rng.Read(zone))
			must(rng.Read(pk))
			must(rng.Read(sk))
			must(rng.Read(cidr[:3]))

			if rng.Uint64()%6 == 0 {
				minRange := 1 + (rng.Uint64() % nodeCount)
				maxRange := max(minRange, (rng.Uint64() % nodeCount))
				count = int(maxRange)
				nodes = nodes[:count]
				autoscaler = &spec.AutoscalerConf{
					Min: int32(minRange),
					Max: int32(maxRange),
				}
			}

			controlNodepools = append(controlNodepools, CreateFakeNodePool(
				WithNodePoolDynamicType(CreateFakeDynamicNodePool(
					WithDynamicNodePoolServerType(string(serverType)),
					WithDynamicNodePoolImage(string(image)),
					WithDynamicNodePoolStorageDiskSize(int32(uint32(rng.Uint64()))),
					WithDynamicNodePoolRegion(string(region)),
					WithDynamicNodePoolZone(string(zone)),
					WithDynamicNodePoolCount(int32(count)),
					WithDynamicNodePoolProvider(GenerateFakeProvider()),
					WithDynamicNodePoolAutoscaler(autoscaler),
					WithDynamicNodePoolMachineSpec(nil),
					WithDynamicNodePoolPublicKey(string(pk)),
					WithDynamicNodePoolPrivateKey(string(sk)),
					WithDynamicNodePoolCIDR(string(cidr)+"/24"),
				)),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		}
	}

	for range cap(workerNodepools) {
		var (
			nodeCount = max(15, rng.Uint64()%255)
			nodes     = make([]*spec.Node, 0, nodeCount)
			name      = make([]byte, 50)
		)

		must(rng.Read(name))

		for range cap(nodes) {
			var (
				name     = make([]byte, 50)
				username = make([]byte, 50)
			)
			must(rng.Read(name))
			must(rng.Read(username))

			if !privateNetwork.Contains(privateStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", privateStartAddress, privateNetwork))
			}
			if !publicNetwork.Contains(publicStartAddress) {
				panic(fmt.Sprintf("failed to generate nodes %s does not belong in %s", publicStartAddress, publicNetwork))
			}

			nodes = append(nodes, CreateFakeNode(
				WithNodeName(string(name)),
				WithNodePrivate(privateStartAddress.String()),
				WithNodePublic(publicStartAddress.String()),
				WithNodeType(spec.NodeType_worker),
				WithNodeUsername(string(name)),
			))
			privateStartAddress = privateStartAddress.Next()
			publicStartAddress = publicStartAddress.Next()
		}

		if rng.Uint64()%2 == 0 { // static
			keys := make(map[string]string)
			for _, n := range nodes {
				key := make([]byte, 256)
				must(rng.Read(key))
				keys[n.Public] = string(key)
			}

			workerNodepools = append(workerNodepools, CreateFakeNodePool(
				WithNodePoolStaticType(CreateFakeStaticNodePool(WithStaticNodePoolNodeKeys(keys))),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		} else { // dynamic
			var (
				serverType = make([]byte, 50)
				image      = make([]byte, 50)
				region     = make([]byte, 50)
				zone       = make([]byte, 50)
				pk         = make([]byte, 50)
				sk         = make([]byte, 50)
				cidr       = make([]byte, 4)
				autoscaler *spec.AutoscalerConf
				count      = len(nodes)
			)

			must(rng.Read(serverType))
			must(rng.Read(image))
			must(rng.Read(region))
			must(rng.Read(zone))
			must(rng.Read(pk))
			must(rng.Read(sk))
			must(rng.Read(cidr[:3]))

			if rng.Uint64()%6 == 0 {
				minRange := 1 + (rng.Uint64() % nodeCount)
				maxRange := max(minRange, (rng.Uint64() % nodeCount))
				count = int(maxRange)
				nodes = nodes[:count]
				autoscaler = &spec.AutoscalerConf{
					Min: int32(minRange),
					Max: int32(maxRange),
				}
			}

			workerNodepools = append(workerNodepools, CreateFakeNodePool(
				WithNodePoolDynamicType(CreateFakeDynamicNodePool(
					WithDynamicNodePoolServerType(string(serverType)),
					WithDynamicNodePoolImage(string(image)),
					WithDynamicNodePoolStorageDiskSize(int32(rng.Uint64())),
					WithDynamicNodePoolRegion(string(region)),
					WithDynamicNodePoolZone(string(zone)),
					WithDynamicNodePoolCount(int32(count)),
					WithDynamicNodePoolProvider(GenerateFakeProvider()),
					WithDynamicNodePoolAutoscaler(autoscaler),
					WithDynamicNodePoolMachineSpec(nil),
					WithDynamicNodePoolPublicKey(string(pk)),
					WithDynamicNodePoolPrivateKey(string(sk)),
					WithDynamicNodePoolCIDR(string(cidr)+"/24"),
				)),
				WithNodePoolName(string(name)),
				WithNodePoolControl(true),
				WithNodePoolNodes(nodes),
			))
		}
	}

	must(rng.Read(name))
	must(rng.Read(hash))

	if !willHaveLbApiEndpoint {
		controlNodepools[int(uint32(rng.Uint64()))%len(controlNodepools)].Nodes[0].NodeType = spec.NodeType_apiEndpoint
	}

	return CreateFakeClusterInfo(
		WithClusterInfoName(string(name)),
		WithClusterInfoHash(string(hash)),
		WithClusterInfoNodePools(append(controlNodepools, workerNodepools...)),
	)
}

func GenerateFakeRoles(willHaveLbApiEndpoint bool, k8s *spec.ClusterInfo) []*spec.Role {
	roles := make([]*spec.Role, 0, 50)

	for range cap(roles) {
		name := make([]byte, 50)
		must(rng.Read(name))

		var protocol string
		if rng.Uint64()%2 == 0 {
			protocol = "tcp"
		} else {
			protocol = "udp"
		}

		var targetPool []string
		for range 1 + rng.Uint64()%20 {
			np := k8s.GetNodePools()[int(uint32(rng.Uint64()))%len(k8s.GetNodePools())]
			targetPool = append(targetPool, np.GetName())
		}

		roles = append(roles, &spec.Role{
			Name:        string(name),
			Protocol:    protocol,
			Port:        int32(rng.Uint64() % math.MaxUint16),
			TargetPort:  int32(rng.Uint64() % math.MaxUint16),
			TargetPools: targetPool,
			RoleType:    spec.RoleType_Ingress,
			Settings: &spec.Role_Settings{
				ProxyProtocol:  false,
				StickySessions: false,
				EnvoyAdminPort: -1,
			},
		})
	}

	if willHaveLbApiEndpoint {
		ep := roles[int(uint32(rng.Uint64()))%len(roles)]
		ep.RoleType = spec.RoleType_ApiServer
		ep.TargetPort = 6443
		ep.Port = 6443
		ep.TargetPools = []string{slices.Collect(nodepools.Control(k8s.GetNodePools()))[0].GetName()}
	}

	return roles
}

func GenerateFakeLBCluster(willHaveLbApiEndpoint bool, k8s *spec.ClusterInfo) *spec.LBcluster {
	var (
		privateNetwork = "192.168.0.0/16"
		publicNetwork  = "10.1.0.0/16"
	)

	return CreateFakeLBCluster(
		WithLBClusterInfo(GenerateFakeLBClusterInfo(privateNetwork, publicNetwork)),
		WithLBclusterRoles(GenerateFakeRoles(willHaveLbApiEndpoint, k8s)),
		WithLBclusterDNS(GenerateFakeDNS()),
		WithLBclusterTargetK8S(k8s.GetName()),
		WithLBclusterUsedApiEndpoint(willHaveLbApiEndpoint),
	)
}

func GenerateFakeK8SCluster(willHaveLbApiEndpoint bool) *spec.K8Scluster {
	var (
		privateNetwork = "192.168.0.0/16"
		publicNetwork  = "10.1.0.0/16"
		kubeconfig     = make([]byte, 50)
	)
	must(rng.Read(kubeconfig))

	return CreateFakeK8SCluster(
		WithK8SClusterInfo(GenerateFakeK8SClusterInfo(willHaveLbApiEndpoint, privateNetwork, publicNetwork)),
		WithK8SNetwork(privateNetwork),
		WithK8SKubeconfig(string(kubeconfig)),
		WithK8SKubernetes("v1.34.0"),
		WithK8SInstallationProxy(nil),
	)
}

type NodeFilter uint8

const (
	NodesAll NodeFilter = iota
	NodesDynamic
	NodesDynamicSkipApiServer
	NodesStatic
)

func AddNodes(count int, ci *spec.ClusterInfo, typ NodeFilter) map[string][]string {
	affected := make(map[string][]string)

	dyn := nodepools.Dynamic(ci.NodePools)
	stat := nodepools.Static(ci.NodePools)

	if len(dyn) == 0 || len(stat) == 0 {
		return nil
	}

	for range count {
		nodeName := make([]byte, 50)
		public := make([]byte, 50)
		private := make([]byte, 50)
		key := make([]byte, 50)

		must(rng.Read(nodeName))
		must(rng.Read(public))
		must(rng.Read(private))
		must(rng.Read(key))

		var np *spec.NodePool
		switch typ {
		case NodesDynamic:
			np = dyn[int(uint32(rng.Uint64()))%len(dyn)]
		case NodesStatic:
			np = stat[int(uint32(rng.Uint64()))%len(stat)]
		case NodesAll:
			np = ci.NodePools[int(uint32(rng.Uint64()))%len(ci.NodePools)]
		case NodesDynamicSkipApiServer:
			np = dyn[int(uint32(rng.Uint64()))%len(dyn)]
			for {
				if _, node := nodepools.FindApiEndpoint([]*spec.NodePool{np}); node == nil {
					break
				}
				np = dyn[int(uint32(rng.Uint64()))%len(dyn)]
			}
		}

		if np.GetDynamicNodePool() != nil {
			np.GetDynamicNodePool().Count += 1
		} else {
			np.GetStaticNodePool().NodeKeys[string(public)] = string(key)
		}

		typ := spec.NodeType_master
		if !np.IsControl {
			typ = spec.NodeType_worker
		}

		np.Nodes = append(np.Nodes, &spec.Node{
			Name:     string(nodeName),
			Private:  string(private),
			Public:   string(public),
			NodeType: typ,
		})
		affected[np.Name] = append(affected[np.Name], string(nodeName))
	}

	return affected
}

// DeleteNodes tries to delete up to `count` number of nodes but may deleted less.
func DeleteNodes(count int, ci *spec.ClusterInfo, typ NodeFilter) ([]*spec.NodePool, map[string][]string) {
	affected := make(map[string]*spec.NodePool)
	counts := make(map[string][]string)

	for range count {
		if len(ci.NodePools) == 0 {
			continue
		}

		np := ci.NodePools[int(uint32(rng.Uint64()))%len(ci.NodePools)]
		if len(np.Nodes) == 0 {
			continue
		}

		switch typ {
		case NodesDynamic:
			if np.GetDynamicNodePool() == nil {
				continue
			}
		case NodesStatic:
			if np.GetStaticNodePool() == nil {
				continue
			}
		case NodesDynamicSkipApiServer:
			for {
				if np.GetDynamicNodePool() == nil {
					np = ci.NodePools[int(uint32(rng.Uint64()))%len(ci.NodePools)]
					continue
				}
				if _, node := nodepools.FindApiEndpoint([]*spec.NodePool{np}); node != nil {
					np = ci.NodePools[int(uint32(rng.Uint64()))%len(ci.NodePools)]
					continue
				}
				break
			}
		}
		node := np.Nodes[int(uint32(rng.Uint64()))%len(np.Nodes)]
		ci.NodePools = nodepools.DeleteNodeByName(ci.NodePools, node.Name, nil)
		affected[np.Name] = np
		counts[np.Name] = append(counts[np.Name], node.Name)
	}

	return slices.Collect(maps.Values(affected)), counts
}

// DeleteNodePools tries to delete up to `count` number of nodepools but may delete less.
func DeleteNodePools(count int, ci *spec.ClusterInfo) ([]*spec.NodePool, map[string][]string) {
	affected := make(map[string]*spec.NodePool)
	counts := make(map[string][]string)

	for range count {
		if len(ci.NodePools) == 0 {
			continue
		}

		nodepool := ci.NodePools[int(uint32(rng.Uint64()))%len(ci.NodePools)]
		var nodes []string
		for _, node := range nodepool.Nodes {
			nodes = append(nodes, node.Name)
		}
		affected[nodepool.Name] = nodepool
		counts[nodepool.Name] = nodes
		ci.NodePools = nodepools.DeleteByName(ci.NodePools, nodepool.Name)
	}

	return slices.Collect(maps.Values(affected)), counts
}

func must[T any](a T, err error) T {
	if err != nil {
		panic(err)
	}
	return a
}
