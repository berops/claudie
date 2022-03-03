package loadbalancer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/clusterBuilder"
	"github.com/Berops/platform/services/terraformer/server/templates"
	"github.com/Berops/platform/services/terraformer/server/terraform"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

const (
	hostnameHashLength int = 15
)

type DNSData struct {
	ClusterName  string
	ClusterHash  string
	HostnameHash string
	DNSZone      string
	NodeIPs      []string
	Project      string
	Provider     *pb.Provider
}

type outputDomain struct {
	Domain map[string]string `json:"-"`
}
type LBcluster struct {
	DesiredLB   *pb.LBcluster
	CurrentLB   *pb.LBcluster
	ProjectName string
}

func (l LBcluster) Build() error {
	var currentInfo *pb.ClusterInfo
	// check if current cluster was defined, to avoid access of unrefferenced memory
	if l.CurrentLB != nil {
		currentInfo = l.CurrentLB.ClusterInfo
	}
	cl := clusterBuilder.ClusterBuilder{
		DesiredInfo: l.DesiredLB.ClusterInfo,
		CurrentInfo: currentInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}
	err := cl.CreateNodepools()
	if err != nil {
		return fmt.Errorf("error while creating the K8s cluster %s : %v", l.DesiredLB.ClusterInfo.Name, err)
	}
	l.createDNS()
	return nil
}

func (l LBcluster) Destroy() error {
	cluster := clusterBuilder.ClusterBuilder{
		//DesiredInfo: , //desired state is not used in DestroyNodepools
		CurrentInfo: l.CurrentLB.ClusterInfo,
		ProjectName: l.ProjectName,
		ClusterType: pb.ClusterType_LB}
	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %v", l.DesiredLB.ClusterInfo.Name, err)
	}
	return nil
}

func (l LBcluster) GetName() string {
	return l.CurrentLB.ClusterInfo.Name
}

func (l LBcluster) createDNS() error {
	clusterID := fmt.Sprintf("%s-%s", l.DesiredLB.ClusterInfo.Name, l.DesiredLB.ClusterInfo.Hash)
	clusterDir := filepath.Join(clusterBuilder.Output, clusterID)
	terraform := terraform.Terraform{Directory: clusterDir}
	templates := templates.Templates{Directory: clusterDir}

	hostnameHash := utils.CreateHash(hostnameHashLength)
	nodeIPs := getNodeIPs(l.DesiredLB.ClusterInfo.NodePools)
	dnsData := getDNSData(l.DesiredLB, hostnameHash, nodeIPs)
	err := templates.Generate("dns.tpl", "dns.tf", dnsData)
	if err != nil {
		return err
	}
	err = terraform.TerraformInit()
	if err != nil {
		return err
	}
	err = terraform.TerraformApply()
	if err != nil {
		return err
	}

	// save full hostname to LBcluster.DNS.Hostname
	outputID := fmt.Sprintf("%s-%s-%s", l.DesiredLB.ClusterInfo.Name, l.DesiredLB.ClusterInfo.Hash, "endpoint")
	output, err := terraform.TerraformOutput(outputID)
	if err != nil {
		log.Error().Msgf("Error while getting output from terraform: %v", err)
		return err
	}
	out, err := readDomain(output)
	if err != nil {
		log.Error().Msgf("Error while reading the terraform output: %v", err)
		return err
	}
	domain := validateDomain(out.Domain[outputID])
	l.DesiredLB.Dns.Hostname = domain
	log.Info().Msgf("Set the domain for %s to %s", l.DesiredLB.ClusterInfo.Name, domain)
	return nil
}

// function returns pair of strings, first the hash hostname, second the zone
func getDNSData(lbCluster *pb.LBcluster, hostname string, nodeIPs []string) DNSData {
	DNSData := DNSData{
		DNSZone:      lbCluster.Dns.DnsZone,
		HostnameHash: hostname,
		ClusterName:  lbCluster.ClusterInfo.Name,
		ClusterHash:  lbCluster.ClusterInfo.Hash,
		NodeIPs:      nodeIPs,
		Project:      lbCluster.Dns.Project,
		Provider:     lbCluster.Dns.Provider,
	}
	return DNSData
}

func getNodeIPs(nodepools []*pb.NodePool) []string {
	var ips []string
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			ips = append(ips, node.Public)
		}
	}
	return ips
}

func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

func readDomain(data string) (outputDomain, error) {
	var result outputDomain
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}
