package spectesting

import (
	"github.com/berops/claudie/proto/pb/spec"
)

type (
	FakeRoleOption                 func(r *spec.Role)
	FakeDnsOption                  func(d *spec.DNS)
	FakeInstalltionProxyOption     func(ip *spec.InstallationProxy)
	FakeProviderGenesisCloudOption func(gcp *spec.Provider_Genesiscloud)
	FakeProviderCloudflareOption   func(gcp *spec.Provider_Cloudflare)
	FakeProviderAzureOption        func(gcp *spec.Provider_Azure)
	FakeProviderAWSOption          func(gcp *spec.Provider_Aws)
	FakeProviderOciOption          func(gcp *spec.Provider_Oci)
	FakeProviderHetznerdnsOption   func(gcp *spec.Provider_Hetznerdns)
	FakeProviderHetznerOption      func(gcp *spec.Provider_Hetzner)
	FakeProviderGCPOption          func(gcp *spec.Provider_Gcp)
	FakeRepositoryOption           func(t *spec.TemplateRepository)
	FakeProviderOption             func(a *spec.Provider)
	FakeAutoscalerOption           func(a *spec.AutoscalerConf)
	FakeMachineSpecOption          func(s *spec.MachineSpec)
	FakeDynamicNodePoolOption      func(dyn *spec.DynamicNodePool)
	FakeStaticNodePoolOption       func(st *spec.StaticNodePool)
	FakeNodeOption                 func(node *spec.Node)
	FakeNodePoolOption             func(np *spec.NodePool)
	FakeClusterInfoOption          func(ci *spec.ClusterInfo)
	FakeK8SOption                  func(k *spec.K8Scluster)
	FakeLBOption                   func(l *spec.LBcluster)
)

func WithRoleName(n string) FakeRoleOption {
	return func(r *spec.Role) {
		r.Name = n
	}
}

func WithRoleProtocol(n string) FakeRoleOption {
	return func(r *spec.Role) {
		r.Protocol = n
	}
}

func WithRolePort(n int32) FakeRoleOption {
	return func(r *spec.Role) {
		r.Port = n
	}
}

func WithRoleTargetPort(n int32) FakeRoleOption {
	return func(r *spec.Role) {
		r.TargetPort = n
	}
}

func WithRoleTargetPools(n []string) FakeRoleOption {
	return func(r *spec.Role) {
		r.TargetPools = n
	}
}

func WithRoleType(t spec.RoleType) FakeRoleOption {
	return func(r *spec.Role) {
		r.RoleType = t
	}
}

func WithDNSZone(zone string) FakeDnsOption {
	return func(d *spec.DNS) {
		d.DnsZone = zone
	}
}

func WithDNSHostname(n string) FakeDnsOption {
	return func(d *spec.DNS) {
		d.Hostname = n
	}
}

func WithDNSProvider(p *spec.Provider) FakeDnsOption {
	return func(d *spec.DNS) {
		d.Provider = p
	}
}

func WithDNSEndpoint(e string) FakeDnsOption {
	return func(d *spec.DNS) {
		d.Endpoint = e
	}
}

func WithLBclusterUsedApiEndpoint(b bool) FakeLBOption {
	return func(l *spec.LBcluster) {
		l.UsedApiEndpoint = b
	}
}

func WithLBclusterTargetK8S(k string) FakeLBOption {
	return func(l *spec.LBcluster) {
		l.TargetedK8S = k
	}
}

func WithLBclusterDNS(dns *spec.DNS) FakeLBOption {
	return func(l *spec.LBcluster) {
		l.Dns = dns
	}
}

func WithLBclusterRoles(roles []*spec.Role) FakeLBOption {
	return func(l *spec.LBcluster) {
		l.Roles = roles
	}
}

func WithLBClusterInfo(ci *spec.ClusterInfo) FakeLBOption {
	return func(l *spec.LBcluster) {
		l.ClusterInfo = ci
	}
}

func WithInstallationProxyMode(m string) FakeInstalltionProxyOption {
	return func(ip *spec.InstallationProxy) {
		ip.Mode = m
	}
}

func WithInstallationProxyEndpoint(m string) FakeInstalltionProxyOption {
	return func(ip *spec.InstallationProxy) {
		ip.Endpoint = m
	}
}

func WithProviderGenesisCloudToken(k string) FakeProviderGenesisCloudOption {
	return func(a *spec.Provider_Genesiscloud) {
		a.Genesiscloud.Token = k
	}
}

func WithProviderCloudflareToken(k string) FakeProviderCloudflareOption {
	return func(a *spec.Provider_Cloudflare) {
		a.Cloudflare.Token = k
	}
}

func WithProviderAzureSubId(k string) FakeProviderAzureOption {
	return func(a *spec.Provider_Azure) {
		a.Azure.SubscriptionID = k
	}
}

func WithProviderAzureTenantId(k string) FakeProviderAzureOption {
	return func(a *spec.Provider_Azure) {
		a.Azure.TenantID = k
	}
}

func WithProviderAzureClientId(k string) FakeProviderAzureOption {
	return func(a *spec.Provider_Azure) {
		a.Azure.ClientID = k
	}
}

func WithProviderAzureClientSecret(k string) FakeProviderAzureOption {
	return func(a *spec.Provider_Azure) {
		a.Azure.ClientSecret = k
	}
}

func WithProviderAWSSecretKey(k string) FakeProviderAWSOption {
	return func(a *spec.Provider_Aws) {
		a.Aws.SecretKey = k
	}
}

func WithProviderAWSAccessKey(k string) FakeProviderAWSOption {
	return func(a *spec.Provider_Aws) {
		a.Aws.AccessKey = k
	}
}

func WithProviderOciID(id string) FakeProviderOciOption {
	return func(a *spec.Provider_Oci) {
		a.Oci.UserOCID = id
	}
}

func WithProviderOciTenancy(id string) FakeProviderOciOption {
	return func(a *spec.Provider_Oci) {
		a.Oci.TenancyOCID = id
	}
}

func WithProviderOciKeyFingerprint(id string) FakeProviderOciOption {
	return func(a *spec.Provider_Oci) {
		a.Oci.KeyFingerprint = id
	}
}

func WithProviderOciCompartmentID(id string) FakeProviderOciOption {
	return func(a *spec.Provider_Oci) {
		a.Oci.CompartmentOCID = id
	}
}

func WithProviderOciPrivateKey(id string) FakeProviderOciOption {
	return func(a *spec.Provider_Oci) {
		a.Oci.PrivateKey = id
	}
}

func WithProviderHetznerdnsToken(k string) FakeProviderHetznerdnsOption {
	return func(a *spec.Provider_Hetznerdns) {
		a.Hetznerdns.Token = k
	}
}

func WithProviderHetznerToken(k string) FakeProviderHetznerOption {
	return func(a *spec.Provider_Hetzner) {
		a.Hetzner.Token = k
	}
}

func WithProviderGCPKey(k string) FakeProviderGCPOption {
	return func(a *spec.Provider_Gcp) {
		a.Gcp.Key = k
	}
}

func WithProviderGCPProject(k string) FakeProviderGCPOption {
	return func(a *spec.Provider_Gcp) {
		a.Gcp.Project = k
	}
}

func WithRepository(rep string) FakeRepositoryOption {
	return func(t *spec.TemplateRepository) {
		t.Repository = rep
	}
}

func WithRepositoryTag(tag string) FakeRepositoryOption {
	return func(t *spec.TemplateRepository) {
		t.Tag = &tag
	}
}

func WithRepositoryPath(path string) FakeRepositoryOption {
	return func(t *spec.TemplateRepository) {
		t.Path = path
	}
}

func WithRepositoryCommitHash(hash string) FakeRepositoryOption {
	return func(t *spec.TemplateRepository) {
		t.CommitHash = hash
	}
}

func WithProviderGCP(typ *spec.Provider_Gcp) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderHetzner(typ *spec.Provider_Hetzner) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderHetznerdns(typ *spec.Provider_Hetznerdns) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderOci(typ *spec.Provider_Oci) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderAws(typ *spec.Provider_Aws) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderAzure(typ *spec.Provider_Azure) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderCloudflare(typ *spec.Provider_Cloudflare) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderGenesiscloud(typ *spec.Provider_Genesiscloud) FakeProviderOption {
	return func(a *spec.Provider) {
		a.ProviderType = typ
	}
}

func WithProviderTemplates(templates *spec.TemplateRepository) FakeProviderOption {
	return func(a *spec.Provider) {
		a.Templates = templates
	}
}

func WithProviderCloudProviderName(name string) FakeProviderOption {
	return func(a *spec.Provider) {
		a.CloudProviderName = name
	}
}

func WithProviderSpecName(name string) FakeProviderOption {
	return func(a *spec.Provider) {
		a.SpecName = name
	}
}

func WithAutoscalerMax(m int32) FakeAutoscalerOption {
	return func(a *spec.AutoscalerConf) {
		a.Max = m
	}
}

func WithAutoscalerMin(m int32) FakeAutoscalerOption {
	return func(a *spec.AutoscalerConf) {
		a.Min = m
	}
}

func WithMachineSpecCpuCount(c int32) FakeMachineSpecOption {
	return func(s *spec.MachineSpec) {
		s.CpuCount = c
	}
}

func WithMachineSpecMemory(m int32) FakeMachineSpecOption {
	return func(s *spec.MachineSpec) {
		s.Memory = m
	}
}

func WithDynamicNodePoolServerType(typ string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.ServerType = typ
	}
}

func WithDynamicNodePoolImage(image string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Image = image
	}
}

func WithDynamicNodePoolStorageDiskSize(size int32) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.StorageDiskSize = size
	}
}

func WithDynamicNodePoolRegion(region string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Region = region
	}
}

func WithDynamicNodePoolZone(zone string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Zone = zone
	}
}

func WithDynamicNodePoolCount(count int32) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Count = count
	}
}

func WithDynamicNodePoolProvider(p *spec.Provider) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Provider = p
	}
}

func WithDynamicNodePoolAutoscaler(s *spec.AutoscalerConf) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.AutoscalerConfig = s
	}
}

func WithDynamicNodePoolMachineSpec(s *spec.MachineSpec) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.MachineSpec = s
	}
}

func WithDynamicNodePoolPublicKey(pub string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.PublicKey = pub
	}
}

func WithDynamicNodePoolPrivateKey(sk string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.PrivateKey = sk
	}
}

func WithDynamicNodePoolCIDR(cidr string) FakeDynamicNodePoolOption {
	return func(dyn *spec.DynamicNodePool) {
		dyn.Cidr = cidr
	}
}

func WithStaticNodePoolNodeKeys(keys map[string]string) FakeStaticNodePoolOption {
	return func(st *spec.StaticNodePool) {
		st.NodeKeys = keys
	}
}

func WithNodeName(name string) FakeNodeOption {
	return func(node *spec.Node) {
		node.Name = name
	}
}

func WithNodePrivate(private string) FakeNodeOption {
	return func(node *spec.Node) {
		node.Private = private
	}
}

func WithNodePublic(public string) FakeNodeOption {
	return func(node *spec.Node) {
		node.Public = public
	}
}

func WithNodeType(typ spec.NodeType) FakeNodeOption {
	return func(node *spec.Node) {
		node.NodeType = typ
	}
}

func WithNodeUsername(username string) FakeNodeOption {
	return func(node *spec.Node) {
		node.Username = username
	}
}

func WithNodePoolDynamicType(dyn *spec.NodePool_DynamicNodePool) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Type = dyn
	}
}

func WithNodePoolStaticType(st *spec.NodePool_StaticNodePool) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Type = st
	}
}

func WithNodePoolName(name string) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Name = name
	}
}

func WithNodePoolNodes(nodes []*spec.Node) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Nodes = nodes
	}
}

func WithNodePoolControl(val bool) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.IsControl = val
	}
}

func WithNodePoolLabels(labels map[string]string) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Labels = labels
	}
}

func WithNodePoolTaints(taints []*spec.Taint) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Taints = taints
	}
}

func WithNodePoolAnnotations(annotations map[string]string) FakeNodePoolOption {
	return func(np *spec.NodePool) {
		np.Annotations = annotations
	}
}

func WithClusterInfoName(name string) FakeClusterInfoOption {
	return func(ci *spec.ClusterInfo) {
		ci.Name = name
	}
}

func WithClusterInfoHash(hash string) FakeClusterInfoOption {
	return func(ci *spec.ClusterInfo) {
		ci.Hash = hash
	}
}

func WithClusterInfoNodePools(nps []*spec.NodePool) FakeClusterInfoOption {
	return func(ci *spec.ClusterInfo) {
		ci.NodePools = nps
	}
}

func WithK8SClusterInfo(ci *spec.ClusterInfo) FakeK8SOption {
	return func(k *spec.K8Scluster) {
		k.ClusterInfo = ci
	}
}

func WithK8SNetwork(network string) FakeK8SOption {
	return func(k *spec.K8Scluster) {
		k.Network = network
	}
}

func WithK8SKubeconfig(kubeconfig string) FakeK8SOption {
	return func(k *spec.K8Scluster) {
		k.Kubeconfig = kubeconfig
	}
}

func WithK8SKubernetes(version string) FakeK8SOption {
	return func(k *spec.K8Scluster) {
		k.Kubernetes = version
	}
}

func WithK8SInstallationProxy(p *spec.InstallationProxy) FakeK8SOption {
	return func(k *spec.K8Scluster) {
		k.InstallationProxy = p
	}
}

func CreateFakeK8SCluster(opts ...FakeK8SOption) *spec.K8Scluster {
	k := new(spec.K8Scluster)
	for _, o := range opts {
		o(k)
	}
	return k
}

func CreateFakeInstallationProxy(opts ...FakeInstalltionProxyOption) *spec.InstallationProxy {
	ip := new(spec.InstallationProxy)
	for _, o := range opts {
		o(ip)
	}
	return ip
}

func CreateFakeProviderGenesisCloud(opts ...FakeProviderGenesisCloudOption) *spec.Provider_Genesiscloud {
	gcp := &spec.Provider_Genesiscloud{
		Genesiscloud: &spec.GenesisCloudProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderCloudflare(opts ...FakeProviderCloudflareOption) *spec.Provider_Cloudflare {
	gcp := &spec.Provider_Cloudflare{
		Cloudflare: &spec.CloudflareProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderAzure(opts ...FakeProviderAzureOption) *spec.Provider_Azure {
	gcp := &spec.Provider_Azure{
		Azure: &spec.AzureProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderAWS(opts ...FakeProviderAWSOption) *spec.Provider_Aws {
	gcp := &spec.Provider_Aws{
		Aws: &spec.AWSProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}
func CreateFakeProviderOci(opts ...FakeProviderOciOption) *spec.Provider_Oci {
	gcp := &spec.Provider_Oci{
		Oci: &spec.OCIProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderHetznerdns(opts ...FakeProviderHetznerdnsOption) *spec.Provider_Hetznerdns {
	gcp := &spec.Provider_Hetznerdns{
		Hetznerdns: &spec.HetznerDNSProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderHetzner(opts ...FakeProviderHetznerOption) *spec.Provider_Hetzner {
	gcp := &spec.Provider_Hetzner{
		Hetzner: &spec.HetznerProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeProviderGCP(opts ...FakeProviderGCPOption) *spec.Provider_Gcp {
	gcp := &spec.Provider_Gcp{
		Gcp: &spec.GCPProvider{},
	}
	for _, o := range opts {
		o(gcp)
	}
	return gcp
}

func CreateFakeRepository(opts ...FakeRepositoryOption) *spec.TemplateRepository {
	r := new(spec.TemplateRepository)
	for _, o := range opts {
		o(r)
	}
	return r
}

func CreateFakeProvider(opts ...FakeProviderOption) *spec.Provider {
	p := new(spec.Provider)
	for _, o := range opts {
		o(p)
	}
	return p
}

func CreateFakeAutoscaler(opts ...FakeAutoscalerOption) *spec.AutoscalerConf {
	a := new(spec.AutoscalerConf)
	for _, o := range opts {
		o(a)
	}
	return a
}

func CreateFakeMachineSpec(opts ...FakeMachineSpecOption) *spec.MachineSpec {
	s := new(spec.MachineSpec)
	for _, o := range opts {
		o(s)
	}
	return s
}

func CreateFakeDynamicNodePool(opts ...FakeDynamicNodePoolOption) *spec.NodePool_DynamicNodePool {
	dyn := &spec.NodePool_DynamicNodePool{
		DynamicNodePool: &spec.DynamicNodePool{},
	}
	for _, o := range opts {
		o(dyn.DynamicNodePool)
	}
	return dyn
}

func CreateFakeStaticNodePool(opts ...FakeStaticNodePoolOption) *spec.NodePool_StaticNodePool {
	st := &spec.NodePool_StaticNodePool{
		StaticNodePool: &spec.StaticNodePool{},
	}
	for _, o := range opts {
		o(st.StaticNodePool)
	}
	return st
}

func CreateFakeNode(opts ...FakeNodeOption) *spec.Node {
	n := new(spec.Node)
	for _, o := range opts {
		o(n)
	}
	return n
}

func CreateFakeNodePool(opts ...FakeNodePoolOption) *spec.NodePool {
	np := new(spec.NodePool)
	for _, o := range opts {
		o(np)
	}
	return np
}

func CreateFakeClusterInfo(opts ...FakeClusterInfoOption) *spec.ClusterInfo {
	ci := new(spec.ClusterInfo)
	for _, o := range opts {
		o(ci)
	}
	return ci
}

func CreateFakeLBCluster(opts ...FakeLBOption) *spec.LBcluster {
	c := new(spec.LBcluster)
	for _, o := range opts {
		o(c)
	}
	return c
}

func CreateFakeDNS(opts ...FakeDnsOption) *spec.DNS {
	d := new(spec.DNS)
	for _, o := range opts {
		o(d)
	}
	return d
}

func CreateFakeRole(opts ...FakeRoleOption) *spec.Role {
	d := new(spec.Role)
	for _, o := range opts {
		o(d)
	}
	return d
}
