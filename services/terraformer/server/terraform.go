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
)

const outputPath string = "services/terraformer/server/terraform"
const templatePath string = "services/terraformer/templates"

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

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(config *pb.Config) error {
	desiredState := config.DesiredState
	log.Info().Msg("Generating templates for infrastructure build")
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	for _, cluster := range desiredState.Clusters {
		log.Info().Msgf("Cluster name: %s", cluster.GetName())
		backendData.ClusterName = cluster.GetName()
		// Creating backend.tf file from the template
		templateFile := filepath.Join(templatePath, "backend.tpl")
		trfFile := filepath.Join(outputPath, "backend.tf")
		if err := templateGen(templateFile, trfFile, backendData, outputPath); err != nil {
			log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
				trfFile, templateFile, err)
			return err
		}
		// Creating .tf files for providers from templates
		if err := buildNodePools(cluster); err != nil {
			return err
		}
		// Create publicKey file for a cluster
		if err := utils.CreateKeyFile(cluster.GetPublicKey(), outputPath, "public.pem"); err != nil {
			return err
		}

		if err := utils.CreateKeyFile(cluster.GetPublicKey(), outputPath, "private.pem"); err != nil {
			return err
		}
		// Call terraform init and apply
		log.Info().Msgf("Running terraform init in %s", outputPath)
		if err := initTerraform(outputPath); err != nil {
			log.Error().Msgf("Error running terraform init in %s: %v", outputPath, err)
			return err
		}

		log.Info().Msgf("Running terraform apply in %s", outputPath)
		if err := applyTerraform(outputPath); err != nil {
			log.Error().Msgf("Error running terraform apply in %s: %v", outputPath, err)
			return err
		}

		// Fill public ip addresses to NodeInfos
		tmpCluster := utils.GetClusterByName(cluster.Name, config.CurrentState.Clusters)
		var m []*pb.NodeInfo
		var newM []*pb.NodeInfo

		if tmpCluster != nil {
			m = tmpCluster.NodeInfos
		}
		for _, nodepool := range cluster.NodePools {
			output, err := outputTerraform(outputPath, nodepool.Provider.Name)
			if err != nil {
				return err
			}

			out, err := readOutput(output)
			if err != nil {
				return err
			}
			res := fillNodes(m, &out, nodepool)
			newM = append(newM, res...)
		}
		cluster.NodeInfos = newM
		// Clean after Terraform. Remove tmp terraform dir.
		err := os.RemoveAll(outputPath)
		if err != nil {
			return err
		}

		for _, m := range desiredState.Clusters {
			for _, nodeInfo := range m.NodeInfos {
				fmt.Println(nodeInfo)
			}
		}
	}
	return nil
}

// destroyInfrastructure executes terraform destroy --auto-approve. It destroys whole infrastructure in a project.
func destroyInfrastructure(project *pb.Project) error {
	log.Info().Msg("Generating templates for infrastructure destroy")
	var backendData Backend
	backendData.ProjectName = project.GetName()
	for _, cluster := range project.Clusters {
		log.Info().Msgf("Cluster name: %s", cluster.GetName())
		backendData.ClusterName = cluster.GetName()
		// Creating backend.tf file
		templateFile := filepath.Join(templatePath, "backend.tpl")
		trfFile := filepath.Join(outputPath, "backend.tf")
		if err := templateGen(templateFile, trfFile, backendData, outputPath); err != nil {
			log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
				trfFile, templateFile, err)
			return err
		}
		// Creating .tf files for providers
		if err := buildNodePools(cluster); err != nil {
			log.Error().Msgf("Error building NodePools for cluster %s : %v", cluster.GetName(), err)
			return err
		}
		// Create publicKey file for a cluster
		if err := utils.CreateKeyFile(cluster.GetPublicKey(), outputPath, "public.pem"); err != nil {
			log.Error().Msgf("Error in CreateKeyFile: %v", err)
			return err
		}
		// Call terraform
		if err := initTerraform(outputPath); err != nil {
			log.Error().Msgf("Error in initTerraform: %v", err)
			return err
		}

		if err := destroyTerraform(outputPath); err != nil {
			log.Error().Msgf("Error in destroyTerraform: %v", err)
			return err
		}

		if err := os.RemoveAll(outputPath); err != nil {
			return err
		}
	}

	return nil
}

// buildNodePools creates .tf files from providers contained in a cluster
func buildNodePools(cluster *pb.Cluster) error {
	for i, nodePool := range cluster.NodePools {
		providerName := nodePool.Provider.Name
		switch providerName {
		case "hetzner", "gcp":
			log.Info().Msgf("Cluster provider: %s", providerName)
			// creating terraform file for a provider
			tplFileName := fmt.Sprintf("%s.tpl", providerName)
			terraFormFileName := fmt.Sprintf("%s.tf", providerName)
			tplFile := filepath.Join(templatePath, tplFileName)
			trfFile := filepath.Join(outputPath, terraFormFileName)
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
			//nodes = readTerraformOutput(nodes)
		default:
			log.Error().Str("provider", providerName).Msgf("Received unsupported provider %s", providerName)
		}
	}

	return nil
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, outputPath string, d interface{}, dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.Mkdir(dirName, os.ModePerm); err != nil {
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
	err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "../../../../keys/platform-296509-d6ddeb344e91.json")
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
func outputTerraform(fileName string, provider string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", provider)
	cmd.Dir = fileName
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

// fillNodes gets ip addresses from a terraform output

func fillNodes(mOld []*pb.NodeInfo, terraformOutput *jsonOut, nodepool *pb.NodePool) []*pb.NodeInfo {
	var mNew []*pb.NodeInfo
	// Create empty slices for node names
	var keysControl []string
	var keysCompute []string
	// Fill slices from terraformOutput maps with names of nodes to ensure an order
	for name := range terraformOutput.Control {
		keysControl = append(keysControl, name)
	}
	sort.Strings(keysControl)
	for name := range terraformOutput.Compute {
		keysCompute = append(keysCompute, name)
	}
	sort.Strings(keysCompute)

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
	return nil, fmt.Errorf("ip does not exist")
}
