package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	outputPath   string = "services/terraformer/server/terraform"
	templatePath string = "services/terraformer/templates"
)

// Backend struct
type Backend struct {
	ProjectName string
	ClusterName string
}

// Data struct
type Data struct {
	Index   int
	Cluster *pb.Cluster
}

type jsonOut struct {
	Compute map[string]string `json:"compute"`
	Control map[string]string `json:"control"`
}

func buildInfrastructureAsync(cluster *pb.Cluster, backendData Backend) error {
	// Prepare backend data for golang templates
	backendData.ClusterName = cluster.GetName()
	log.Info().Msgf("Cluster name: %s", cluster.GetName())

	templateFilePath := filepath.Join(templatePath, "backend.tpl")
	tfFilePath := filepath.Join(outputPath, cluster.GetName(), "backend.tf")
	outputPathCluster := filepath.Join(outputPath, cluster.GetName())

	// Creating backend.tf file from the template backend.tpl
	if err := templateGen(templateFilePath, tfFilePath, backendData, outputPathCluster); err != nil {
		log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
			tfFilePath, templateFilePath, err)
		return err
	}

	// Creating .tf files for providers from templates
	if err := buildNodePools(cluster, outputPathCluster); err != nil {
		return err
	}

	// Create publicKey and privateKey file for a cluster
	terraformOutputPath := filepath.Join(outputPath, cluster.GetName())
	if err := utils.CreateKeyFile(cluster.GetPublicKey(), terraformOutputPath, "public.pem"); err != nil {
		return err
	}
	if err := utils.CreateKeyFile(cluster.GetPublicKey(), terraformOutputPath, "private.pem"); err != nil {
		return err
	}

	// Call terraform init and apply
	log.Info().Msgf("Running terraform init in %s", terraformOutputPath)
	if err := initTerraform(terraformOutputPath); err != nil {
		log.Error().Msgf("Error running terraform init in %s: %v", terraformOutputPath, err)
		return err
	}
	log.Info().Msgf("Running terraform apply in %s", terraformOutputPath)
	if err := applyTerraform(terraformOutputPath); err != nil {
		log.Error().Msgf("Error running terraform apply in %s: %v", terraformOutputPath, err)
		return err
	}

	// Fill public ip addresses to NodeInfos
	var m []*pb.NodeInfo
	for _, nodepool := range cluster.NodePools {
		output, err := outputTerraform(terraformOutputPath, nodepool.Provider.Name)
		if err != nil {
			return err
		}

		out, err := readOutput(output)
		if err != nil {
			return err
		}

		res := fillNodes(cluster.NodeInfos, &out, nodepool)
		m = append(m, res...)
	}
	cluster.NodeInfos = m

	return nil
}

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(currentState *pb.Project, desiredState *pb.Project) error {
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	var errGroup errgroup.Group

	for _, cluster := range desiredState.GetClusters() {
		func(cluster *pb.Cluster, backendData Backend) {
			errGroup.Go(func() error {
				err := buildInfrastructureAsync(cluster, backendData)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - buildInfrastructure: %v", err)
					return err
				}
				return nil
			})
		}(cluster, backendData)
	}
	err := errGroup.Wait()
	if err != nil {
		return err
	}

	// Clean after terraform
	if err := os.RemoveAll(outputPath); err != nil {
		return err
	}

	return nil
}

// destroyInfrastructureAsync executes terraform destroy --auto-approve. It destroys whole infrastructure in a project.
func destroyInfrastructureAsync(cluster *pb.Cluster, backendData Backend) error {
	log.Info().Msg("Generating templates for infrastructure destroy")
	backendData.ClusterName = cluster.GetName()

	log.Info().Msgf("Cluster name: %s", cluster.GetName())

	// Creating backend.tf file
	templateFilePath := filepath.Join(templatePath, "backend.tpl")
	tfFilePath := filepath.Join(outputPath, cluster.GetName(), "backend.tf")
	outputPathCluster := filepath.Join(outputPath, cluster.GetName())

	// Creating backend.tf file from the template backend.tpl
	if err := templateGen(templateFilePath, tfFilePath, backendData, outputPathCluster); err != nil {
		log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
			tfFilePath, templateFilePath, err)
		return err
	}

	// Creating .tf files for providers from templates
	if err := buildNodePools(cluster, outputPathCluster); err != nil {
		return err
	}

	// Create publicKey and privateKey file for a cluster
	terraformOutputPath := filepath.Join(outputPath, cluster.GetName())
	if err := utils.CreateKeyFile(cluster.GetPublicKey(), terraformOutputPath, "public.pem"); err != nil {
		return err
	}
	if err := utils.CreateKeyFile(cluster.GetPublicKey(), terraformOutputPath, "private.pem"); err != nil {
		return err
	}

	// Call terraform init and apply
	log.Info().Msgf("Running terraform init in %s", terraformOutputPath)
	if err := initTerraform(terraformOutputPath); err != nil {
		log.Error().Msgf("Error running terraform init in %s: %v", terraformOutputPath, err)
		return err
	}

	if err := destroyTerraform(terraformOutputPath); err != nil {
		log.Error().Msgf("Error in destroyTerraform: %v", err)
		return err
	}

	return nil
}

func destroyInfrastructure(config *pb.Config) error {
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = config.GetDesiredState().GetName()
	var errGroup errgroup.Group

	for _, cluster := range config.GetDesiredState().GetClusters() {
		func(cluster *pb.Cluster, backendData Backend) {
			errGroup.Go(func() error {
				err := destroyInfrastructureAsync(cluster, backendData)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - destroyInfrastructure: %v", err)
					config.ErrorMessage = err.Error()
					return err
				}
				return nil
			})
		}(cluster, backendData)
	}
	err := errGroup.Wait()
	if err != nil {
		config.ErrorMessage = err.Error()
		return err
	}

	if err := os.RemoveAll(outputPath); err != nil {
		return err
	}

	return nil
}

// buildNodePools creates .tf files from providers contained in a cluster
func buildNodePools(cluster *pb.Cluster, outputPathCluster string) error {
	for i, nodePool := range cluster.NodePools {
		providerName := nodePool.Provider.Name
		log.Info().Msgf("Cluster provider: %s", providerName)
		// creating terraform file for a provider
		tplFileName := fmt.Sprintf("%s.tpl", providerName)
		terraFormFileName := fmt.Sprintf("%s.tf", providerName)
		tplFile := filepath.Join(templatePath, tplFileName)
		trfFile := filepath.Join(outputPathCluster, terraFormFileName)
		genRes := templateGen(
			tplFile,
			trfFile,
			&Data{Index: i, Cluster: cluster},
			templatePath)
		if genRes != nil {
			log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
				trfFile, tplFile, genRes)
			return genRes
		}
	}

	return nil
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, outputPath string, d interface{}, dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create the %s file: %v", dirName, err)
	}

	if err := tpl.Execute(f, d); err != nil {
		return fmt.Errorf("failed to execute the template file: %v", err)
	}

	return nil
}

// initTerraform executes terraform init in a given path
func initTerraform(directoryName string) error {
	// Apply GCP credentials as an env variable
	err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "../../../../../keys/platform-296509-d6ddeb344e91.json")
	if err != nil {
		return fmt.Errorf("failed to set the google credentials env variable: %v", err)
	}
	// terraform init
	return executeTerraform(exec.Command("terraform", "init"), directoryName)
}

// applyTerraform executes terraform apply on a .tf files in a given path
func applyTerraform(directoryName string) error {
	// terraform apply --auto-approve
	return executeTerraform(exec.Command("terraform", "apply", "--auto-approve"), directoryName)
}

// destroyTerraform executes terraform destroy in a given path
func destroyTerraform(directoryName string) error {
	// terraform destroy
	return executeTerraform(exec.Command("terraform", "destroy", "--auto-approve"), directoryName)
}

func executeTerraform(cmd *exec.Cmd, workingDir string) error {
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// outputTerraform returns terraform output for a given provider and path in a json format
func outputTerraform(dirName string, provider string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", provider)
	cmd.Dir = dirName
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outb.String(), nil
}

// readOutput reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readOutput(data string) (jsonOut, error) {
	var result jsonOut
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

// getKeysFromMap returns an array of all keys in a map
func getkeysFromMap(data map[string]string) []string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// fillNodes gets ip addresses from a terraform output
func fillNodes(mOld []*pb.NodeInfo, terraformOutput *jsonOut, nodepool *pb.NodePool) []*pb.NodeInfo {
	var mNew []*pb.NodeInfo
	// Fill slices from terraformOutput maps with names of nodes to ensure an order
	keysControl := getkeysFromMap(terraformOutput.Control)
	keysCompute := getkeysFromMap(terraformOutput.Compute)

	for _, name := range keysControl {
		var private = ""
		var control uint32 = 1
		// If node exist, assign previous private IP
		existingIP, _ := existsInCluster(mOld, terraformOutput.Control[name])
		if existingIP != nil {
			private = existingIP.Private
			control = existingIP.IsControl
		}
		mNew = append(mNew, &pb.NodeInfo{
			NodeName:     name,
			Public:       terraformOutput.Control[name],
			Private:      private,
			IsControl:    control,
			Provider:     nodepool.Provider.Name,
			NodepoolName: nodepool.Name,
		})
	}
	for _, name := range keysCompute {
		var private = ""
		// If node exist, assign previous private IP
		existingIP, _ := existsInCluster(mOld, terraformOutput.Compute[name])
		if existingIP != nil {
			private = existingIP.Private
		}
		mNew = append(mNew, &pb.NodeInfo{
			NodeName:     name,
			Public:       terraformOutput.Compute[name],
			Private:      private,
			IsControl:    0,
			Provider:     nodepool.Provider.Name,
			NodepoolName: nodepool.Name,
		})
	}
	return mNew
}

func existsInCluster(m []*pb.NodeInfo, ip string) (*pb.NodeInfo, error) {
	for _, ips := range m {
		if ips.Public == ip {
			return ips, nil
		}
	}
	return nil, fmt.Errorf("ip address %v does not exist", ip)
}
