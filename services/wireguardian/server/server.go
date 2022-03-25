package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedWireguardianServiceServer
}

type dataForIniTemplate struct {
	Nodepools         []*pb.NodePool
	LBClusters        []*pb.LBcluster
	DirName           string
	ClusterSSHKeyName string
	PrivateFileExt    string
}

type dataForConfTemplate struct {
	Roles []lbRolesWithNodes
}

type dataForNginxPlaybookTemplate struct {
	ConfPath string
}

type dataForAPIEndpointTemplate struct {
	OldEndpoint string
	NewEndpoint string
}

type lbRolesWithNodes struct {
	*pb.Role
	Nodes []*pb.Node
}

const (
	outputPath               = "services/wireguardian/server/Ansible"
	inventoryTemplate        = "services/wireguardian/server/inventory.goini"
	nginxCongTemplate        = "services/wireguardian/server/conf.gotpl"
	nginxPlaybookTemplate    = "services/wireguardian/server/nginx.goyml"
	apiEndpointPlaybookFile  = "apiEndpointChange.yml"
	longhornPlaybookFile     = "longhorn.yml"
	inventoryFile            = "inventory.ini"
	nginxConfFileExt         = ".conf"
	playbookExt              = ".yml"
	privateFileExt           = ".pem"
	playbookFile             = "playbook.yml"
	sshClusterPrivateKeyFile = "cluster"
	defaultWireguardianPort  = 50053
)

func (*server) RunAnsible(_ context.Context, req *pb.RunAnsibleRequest) (*pb.RunAnsibleResponse, error) {
	desiredState := req.GetDesiredState()
	currentState := req.GetCurrentState()
	var errGroup errgroup.Group
	// Create VPN and set up LB
	for _, cluster := range desiredState.GetClusters() {
		// to pass the parameter in loop, we need to create a dummy function
		func(cluster *pb.K8Scluster) {
			errGroup.Go(func() error {
				err := buildVPNAsync(cluster, desiredState.LoadBalancerClusters, currentState, desiredState)
				if err != nil {
					log.Error().Msgf("error encountered in Wireguardian - BuildVPN: %v", err)
					return err
				}
				return nil
			})
		}(cluster)
	}
	err := errGroup.Wait()
	if err != nil {
		return &pb.RunAnsibleResponse{DesiredState: desiredState}, err
	}
	return &pb.RunAnsibleResponse{DesiredState: desiredState}, nil
}

func buildVPNAsync(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster, currentState, desiredState *pb.Project) error {

	matchingLBClusters := findLBCluster(cluster.ClusterInfo.Name, lbClusters)
	changedEndpoints := getEndpointChanges(currentState, desiredState, matchingLBClusters)
	if err := genPrivAdd(groupNodepool(cluster, matchingLBClusters), cluster.GetNetwork()); err != nil {
		log.Error().Msgf("error while generating private addresses: %v", err)
		return err
	}

	outputPath := filepath.Join(outputPath, cluster.ClusterInfo.GetName()+"-"+cluster.ClusterInfo.GetHash())
	if err := genTpl(cluster, matchingLBClusters, outputPath); err != nil {
		log.Error().Msgf("error while genereting inventory file: %v", err)
		return err
	}

	if err := runAnsible(cluster, matchingLBClusters, changedEndpoints, outputPath); err != nil {
		log.Error().Msgf("error while running Ansible: %v", err)
		return err
	}

	if err := os.RemoveAll(outputPath); err != nil {
		return fmt.Errorf("failed to remove ansible files for cluster: %v", err)
	}

	return nil
}

// genPrivAdd will generate private ip addresses from network parameter
func genPrivAdd(nodepools []*pb.NodePool, network string) error {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR: %v", err)
	}
	var addressesToAssign []*pb.Node

	// initialize slice of possible last octet
	lastOctets := make([]byte, 255)
	var i byte
	for i = 0; i < 255; i++ {
		lastOctets[i] = i + 1
	}

	ip := ipNet.IP
	ip = ip.To4()
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			// If address already assigned, skip
			if node.Private != "" {
				lastOctet := strings.Split(node.Private, ".")[3]
				lastOctetInt, _ := strconv.Atoi(lastOctet)
				lastOctets = remove(lastOctets, byte(lastOctetInt))
				continue
			}
			addressesToAssign = append(addressesToAssign, node)
		}
	}

	var temp int
	for _, address := range addressesToAssign {
		ip[3] = lastOctets[temp]
		address.Private = ip.String()
		temp++
	}
	// debug message
	for _, nodepool := range nodepools {
		fmt.Println(nodepool)
	}

	return nil
}

func remove(slice []byte, value byte) []byte {
	for idx, v := range slice {
		if v == value {
			return append(slice[:idx], slice[idx+1:]...)
		}
	}
	return slice
}

// genTpl will generate ansible inventory file slice of clusters input
func genTpl(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster, outputPath string) error {

	// inventory file
	iniData := &dataForIniTemplate{
		Nodepools:         cluster.ClusterInfo.NodePools,
		LBClusters:        lbClusters,
		DirName:           cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash,
		ClusterSSHKeyName: sshClusterPrivateKeyFile,
		PrivateFileExt:    privateFileExt,
	}
	err := tplExecution(iniData, inventoryTemplate, outputPath, inventoryFile)
	if err != nil {
		return err
	}

	// nginx conf files nad
	controlNodes, computeNodes := nodeSegregation(cluster)
	for _, lbCluster := range lbClusters {

		confData := &dataForConfTemplate{}
		for _, role := range lbCluster.Roles {
			tmpRole := lbRolesWithNodes{Role: role}

			if role.Target == pb.Target_k8sAllNodes {
				tmpRole.Nodes = append(controlNodes, computeNodes...)
			} else if role.Target == pb.Target_k8sControlPlane {
				tmpRole.Nodes = controlNodes
			} else if role.Target == pb.Target_k8sComputePlane {
				tmpRole.Nodes = computeNodes
			}
			confData.Roles = append(confData.Roles, tmpRole)
		}
		err = tplExecution(confData, nginxCongTemplate, outputPath, lbCluster.ClusterInfo.Name+nginxConfFileExt)
		if err != nil {
			return err
		}
	}
	return nil
}

func tplExecution(data interface{}, templateFilePath string, outputPath string, filename string) error {
	tpl, err := template.ParseFiles(templateFilePath)
	if err != nil {
		return fmt.Errorf("failed to load template file:%s, err:%v", templateFilePath, err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		err = os.MkdirAll(outputPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create the directory:%s, err:%v", outputPath, err)
		}
	}

	f, err := os.Create(filepath.Join(outputPath, filename))
	if err != nil {
		return fmt.Errorf("failed to create the file:%s, err:%v", filename, err)
	}

	if err := tpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template file:%s, err:%v", filename, err)
	}

	return nil

}

func runAnsible(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster, changedEndpoint map[string]dataForAPIEndpointTemplate, clusterOutputPath string) error {
	if err := utils.CreateKeyFile(cluster.ClusterInfo.GetPrivateKey(), clusterOutputPath, sshClusterPrivateKeyFile+privateFileExt); err != nil {
		return fmt.Errorf("failed to create key file: %v", err)

	}
	for _, lbCluster := range lbClusters {
		if err := utils.CreateKeyFile(lbCluster.ClusterInfo.GetPrivateKey(), clusterOutputPath, lbCluster.ClusterInfo.Name+privateFileExt); err != nil {
			return fmt.Errorf("failed to create key file for LB cluster %s : %v", lbCluster.ClusterInfo.Name, err)
		}
	}

	if err := os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False"); err != nil {
		return fmt.Errorf("failed to set ANSIBLE_HOST_KEY_CHECKING env var: %v", err)
	}

	inventoryFilePath := cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + inventoryFile

	cmd := exec.Command("ansible-playbook", playbookFile, "-i", inventoryFilePath, "-f", "20", "-l", "nodes")
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	//install longhorn dependencies
	cmd = exec.Command("ansible-playbook", longhornPlaybookFile, "-i", inventoryFilePath, "-f", "20")
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	// generate and run nginx playbook
	for _, lbCluster := range lbClusters {

		// generate nginx playbook and run
		d := dataForNginxPlaybookTemplate{ConfPath: "./" + lbCluster.ClusterInfo.Name + nginxConfFileExt}
		err := tplExecution(d, nginxPlaybookTemplate, clusterOutputPath, lbCluster.ClusterInfo.Name+playbookExt)
		if err != nil {
			return err
		}

		nginxPlaybookPath := cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + lbCluster.ClusterInfo.Name + playbookExt
		cmd := exec.Command("ansible-playbook", nginxPlaybookPath, "-i", inventoryFilePath, "-f", "20", "-l", lbCluster.ClusterInfo.Name)
		cmd.Dir = outputPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}

		// check if apiendpoint is changed
		if d, ok := changedEndpoint[lbCluster.ClusterInfo.Name]; ok {
			// run apiEndpoint playbook
			cmd := exec.Command("ansible-playbook", apiEndpointPlaybookFile, "-i", inventoryFilePath, "-f", "20", "--extra-vars", "NewEndpoint="+d.NewEndpoint+" OldEndpoint="+d.OldEndpoint)
			cmd.Dir = outputPath
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// find the all the load balancer cluster for a K8s cluster
func findLBCluster(ClusterName string, lbClusters []*pb.LBcluster) []*pb.LBcluster {
	var matchingLBClusters []*pb.LBcluster
	for _, lbCluster := range lbClusters {
		if lbCluster.TargetedK8S == ClusterName {
			matchingLBClusters = append(matchingLBClusters, lbCluster)
		}
	}
	return matchingLBClusters
}

func getEndpointChanges(currentState, desiredState *pb.Project, lbClusters []*pb.LBcluster) map[string]dataForAPIEndpointTemplate {
	changedEndpointClusters := make(map[string]dataForAPIEndpointTemplate)
	for _, lbCluster := range lbClusters {
		for _, currentLbCluster := range currentState.LoadBalancerClusters {
			if currentLbCluster.ClusterInfo.Name == lbCluster.ClusterInfo.Name {
				for _, desiredLbCluster := range desiredState.LoadBalancerClusters {
					if desiredLbCluster.ClusterInfo.Name == lbCluster.ClusterInfo.Name {
						if utils.ChangedAPIEndpoint(currentLbCluster.Dns, desiredLbCluster.Dns) {
							changedEndpointClusters[lbCluster.ClusterInfo.Name] = dataForAPIEndpointTemplate{OldEndpoint: currentLbCluster.Dns.Endpoint, NewEndpoint: desiredLbCluster.Dns.Endpoint}
						}
					}
				}
			}
		}
	}
	return changedEndpointClusters
}

func nodeSegregation(cluster *pb.K8Scluster) (controlNodes, ComputeNodes []*pb.Node) {
	for _, nodepools := range cluster.ClusterInfo.NodePools {
		for _, node := range nodepools.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint || node.NodeType == pb.NodeType_master {
				controlNodes = append(controlNodes, node)
			} else {
				ComputeNodes = append(ComputeNodes, node)
			}
		}
	}
	return
}

func groupNodepool(k8sCluster *pb.K8Scluster, lbClusters []*pb.LBcluster) []*pb.NodePool {
	var nodepools []*pb.NodePool
	nodepools = append(nodepools, k8sCluster.ClusterInfo.NodePools...)
	for _, lb := range lbClusters {
		nodepools = append(nodepools, lb.ClusterInfo.NodePools...)
	}
	return nodepools
}
func main() {
	// initialize logger
	utils.InitLog("wireguardian", "GOLANG_LOG")

	// Set Wireguardian port
	wireguardianPort := utils.GetenvOr("WIREGUARDIAN_PORT", fmt.Sprint(defaultWireguardianPort))

	serviceAddr := net.JoinHostPort("0.0.0.0", wireguardianPort)
	lis, err := net.Listen("tcp", serviceAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", serviceAddr, err)
	}
	log.Info().Msgf("Wireguardian service is listening on %s", serviceAddr)

	// creating a new server
	s := grpc.NewServer()
	pb.RegisterWireguardianServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(wireguardianPort, "WIREGUARDIAN_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch

		signal.Stop(ch)
		s.GracefulStop()

		return errors.New("wireguardian Interrupt signal")
	})

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("wireguardian failed to serve: %v", err)
		}
		return nil
	})

	log.Info().Msgf("Stopping Wireguardian: %v", g.Wait())
}
