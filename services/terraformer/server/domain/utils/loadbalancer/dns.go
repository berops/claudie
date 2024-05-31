package loadbalancer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/proto/pb"
	cluster_builder "github.com/berops/claudie/services/terraformer/server/domain/utils/cluster-builder"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/backend"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/provider"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/templates"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/terraform"
)

const (
	TemplatesRootDir = "services/terraformer/templates"
)

type DNS struct {
	ProjectName string
	ClusterName string
	ClusterHash string

	DesiredNodeIPs []string
	CurrentNodeIPs []string

	CurrentDNS *pb.DNS
	DesiredDNS *pb.DNS

	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

type TemplateGeneration struct {
	ProjectName     string
	TargetDirectory string
	ClusterData     templates.ClusterData
	NodeIPs         []string
}

// CreateDNSRecords creates DNS records for the Loadbalancer cluster.
func (d DNS) CreateDNSRecords(logger zerolog.Logger) (string, error) {
	var (
		sublogger       = logger.With().Str("endpoint", d.DesiredDNS.Endpoint).Logger()
		clusterID       = fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
		targetDirectory = filepath.Join(cluster_builder.Output, clusterID, "dns")
	)

	terraform := terraform.Terraform{
		Directory:         targetDirectory,
		SpawnProcessLimit: d.SpawnProcessLimit,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(clusterID)
		terraform.Stderr = comm.GetStdErr(clusterID)
	}

	if changedDNSProvider(d.CurrentDNS, d.DesiredDNS) {
		sublogger.Info().Msg("Destroying old DNS records")

		tg := TemplateGeneration{
			ProjectName:     d.ProjectName,
			TargetDirectory: targetDirectory,
			ClusterData: templates.ClusterData{
				ClusterName: d.ClusterName,
				ClusterHash: d.ClusterHash,
			},
			NodeIPs: d.CurrentNodeIPs,
		}

		if err := generateFiles(d.CurrentDNS, &tg); err != nil {
			return "", fmt.Errorf("error while generating dns templates for %s : %w", clusterID, err)
		}

		if err := terraform.Init(); err != nil {
			return "", err
		}

		if err := terraform.Destroy(); err != nil {
			return "", err
		}

		if err := os.RemoveAll(targetDirectory); err != nil {
			return "", fmt.Errorf("error while removing files in dir %q: %w", targetDirectory, err)
		}

		sublogger.Info().Msg("Old DNS records were successfully destroyed")
	}

	tg := TemplateGeneration{
		ProjectName:     d.ProjectName,
		TargetDirectory: targetDirectory,
		ClusterData: templates.ClusterData{
			ClusterName: d.ClusterName,
			ClusterHash: d.ClusterHash,
		},
		NodeIPs: d.DesiredNodeIPs,
	}

	sublogger.Info().Msg("Creating new DNS records")

	if err := generateFiles(d.DesiredDNS, &tg); err != nil {
		return "", fmt.Errorf("error while creating dns.tf files for %s : %w", clusterID, err)
	}

	if err := terraform.Init(); err != nil {
		return "", err
	}

	if err := terraform.Apply(); err != nil {
		return "", err
	}

	output, err := terraform.Output(clusterID)
	if err != nil {
		return "", fmt.Errorf("error while getting output from terraform for %s : %w", clusterID, err)
	}

	out, err := readDomain(output)
	if err != nil {
		return "", fmt.Errorf("error while reading output from terraform for %s : %w", clusterID, err)
	}

	sublogger.Info().Msg("DNS records were successfully set up")

	outputID := fmt.Sprintf("%s-%s", clusterID, "endpoint")
	if err := os.RemoveAll(targetDirectory); err != nil {
		return validateDomain(out.Domain[outputID]), fmt.Errorf("error while deleting files in %s: %w", targetDirectory, err)
	}
	return validateDomain(out.Domain[outputID]), nil
}

// DestroyDNSRecords destroys DNS records for the Loadbalancer cluster.
func (d DNS) DestroyDNSRecords(logger zerolog.Logger) error {
	var (
		sublogger       = logger.With().Str("endpoint", d.CurrentDNS.Endpoint).Logger()
		clusterID       = fmt.Sprintf("%s-%s", d.ClusterName, d.ClusterHash)
		targetDirectory = filepath.Join(cluster_builder.Output, clusterID, "dns")
	)

	sublogger.Info().Msg("Destroying DNS records")

	tg := TemplateGeneration{
		ProjectName:     d.ProjectName,
		TargetDirectory: targetDirectory,
		ClusterData: templates.ClusterData{
			ClusterName: d.ClusterName,
			ClusterHash: d.ClusterHash,
		},
		NodeIPs: d.CurrentNodeIPs,
	}

	if err := generateFiles(d.CurrentDNS, &tg); err != nil {
		return fmt.Errorf("error while generating dns templates for %q : %w", clusterID, err)
	}

	terraform := terraform.Terraform{
		Directory:         targetDirectory,
		SpawnProcessLimit: d.SpawnProcessLimit,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(clusterID)
		terraform.Stderr = comm.GetStdErr(clusterID)
	}

	if err := terraform.Init(); err != nil {
		return err
	}

	if err := terraform.Destroy(); err != nil {
		return err
	}

	// TODO: fix hetzner provider common resources.
	sublogger.Info().Msg("DNS records were successfully destroyed")

	if err := os.RemoveAll(targetDirectory); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", targetDirectory, err)
	}

	return nil
}

// generateFiles creates all the necessary terraform files used to create/destroy DNS.
func generateFiles(dns *pb.DNS, tg *TemplateGeneration) error {
	clusterID := fmt.Sprintf("%s-%s", tg.ClusterData.ClusterName, tg.ClusterData.ClusterHash)

	b := backend.Backend{
		Key:       fmt.Sprintf("%s/%s/dns", tg.ProjectName, clusterID),
		Directory: tg.TargetDirectory,
	}

	if err := backend.Create(&b); err != nil {
		return err
	}

	if err := provider.CreateDNS(tg.TargetDirectory, dns); err != nil {
		return err
	}

	repo := templates.Repository{TemplatesRootDirectory: TemplatesRootDir}
	if err := repo.Download(dns.GetTemplates()); err != nil {
		return fmt.Errorf("failed to download template repository: %w", err)
	}

	g := templates.DNSGenerator{
		TargetDirectory:   tg.TargetDirectory,
		ReadFromDirectory: TemplatesRootDir,
		DNS:               dns,
	}

	data := templates.DNSData{
		DNSZone:      dns.DnsZone,
		HostnameHash: dns.Hostname,
		ClusterName:  tg.ClusterData.ClusterName,
		ClusterHash:  tg.ClusterData.ClusterHash,
		NodeIPs:      tg.NodeIPs,
		Provider:     dns.Provider,
	}

	if err := g.GenerateDNS(&data); err != nil {
		return fmt.Errorf("failed to generate dns templates: %w", err)
	}

	return nil
}

// validateDomain validates the domain does not start with ".".
func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

// readDomain reads full domain from terraform output.
func readDomain(data string) (templates.DNSDomain, error) {
	var result templates.DNSDomain
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}

func changedDNSProvider(currentDNS, desiredDNS *pb.DNS) bool {
	// DNS not yet created
	if currentDNS == nil {
		return false
	}
	// DNS provider are same
	if currentDNS.Provider.SpecName == desiredDNS.Provider.SpecName {
		if currentDNS.Provider.Credentials == desiredDNS.Provider.Credentials {
			return false
		}
	}
	return true
}
