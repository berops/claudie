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
	Nodepools                []*pb.NodePool
	LBNodepools              []*pb.NodePool
	SshClusterPrivateKeyFile string
	SshLBPrivateKeyFile      string
}

type dataForConfTemplate struct {
	Roles []lbRolesWithNodes
}

type lbRolesWithNodes struct {
	*pb.Role
	Nodes []*pb.Node
}

const (
	outputPath               = "services/wireguardian/server/Ansible"
	inventoryTemplate        = "services/wireguardian/server/inventory.goini"
	nginxCongTemplate        = "services/wireguardian/server/conf.gotpl"
	inventoryFile            = "inventory.ini"
	nginxConfFileExt         = ".conf"
	playbookFile             = "playbook.yml"
	nginxPlaybook            = "nginx.yml"
	sshClusterPrivateKeyFile = "cluster.pem"
	sshLBPrivateKeyFile      = "lb.pem"
	defaultWireguardianPort  = 50053
)

func (*server) BuildVPN(_ context.Context, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	desiredState := req.GetDesiredState()
	var errGroup errgroup.Group

	for _, cluster := range desiredState.GetClusters() {
		// to pass the parameter in loop, we need to create a dummy function
		func(cluster *pb.K8Scluster) {
			errGroup.Go(func() error {
				err := buildVPNAsync(cluster, desiredState.LoadBalancerClusters)
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
		return &pb.BuildVPNResponse{DesiredState: desiredState}, err
	}
	return &pb.BuildVPNResponse{DesiredState: desiredState}, nil
}

func buildVPNAsync(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster) error {

	lbCluster := findLBCluster(cluster.ClusterInfo.Name, lbClusters)

	if err := genPrivAdd(append(cluster.ClusterInfo.NodePools, lbCluster.ClusterInfo.NodePools...), cluster.GetNetwork()); err != nil {
		return err
	}

	outputPath := filepath.Join(outputPath, cluster.ClusterInfo.GetName()+"-"+cluster.ClusterInfo.GetHash())
	if err := genTpl(cluster, lbCluster, outputPath); err != nil {
		return err
	}

	if err := runAnsible(cluster, lbCluster, outputPath); err != nil {
		return err
	}

	if err := os.RemoveAll(outputPath); err != nil {
		return err
	}

	return nil
}

// genPrivAdd will generate private ip addresses from network parameter
func genPrivAdd(nodepools []*pb.NodePool, network string) error {
	_, ipNet, err := net.ParseCIDR(network)
	var addressesToAssign []*pb.Node

	// initialize slice of possible last octet
	lastOctets := make([]byte, 255)
	var i byte
	for i = 0; i < 255; i++ {
		lastOctets[i] = i + 1
	}

	if err != nil {
		return err
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
func genTpl(cluster *pb.K8Scluster, lbCluster *pb.LBcluster, outputPath string) error {

	// inventory file
	iniData := &dataForIniTemplate{
		Nodepools:                cluster.ClusterInfo.NodePools,
		LBNodepools:              lbCluster.ClusterInfo.NodePools,
		SshClusterPrivateKeyFile: cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + sshClusterPrivateKeyFile,
		SshLBPrivateKeyFile:      cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + sshLBPrivateKeyFile,
	}
	err := tplExecution(iniData, inventoryTemplate, outputPath, inventoryFile)
	if err != nil {
		return err
	}

	// nginx conf file
	controlNodes, computeNodes := nodeSegregation(cluster)
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

func runAnsible(cluster *pb.K8Scluster, lbCluster *pb.LBcluster, invOutputPath string) error {
	if err := utils.CreateKeyFile(cluster.ClusterInfo.GetPrivateKey(), invOutputPath, sshClusterPrivateKeyFile); err != nil {
		return err
	}

	if lbCluster != nil {
		if err := utils.CreateKeyFile(lbCluster.ClusterInfo.GetPrivateKey(), invOutputPath, sshLBPrivateKeyFile); err != nil {
			return err
		}
	}

	if err := os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False"); err != nil {
		return err
	}

	inventoryFilePath := cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + inventoryFile
	configFilePath := cluster.ClusterInfo.Name + "-" + cluster.ClusterInfo.Hash + "/" + lbCluster.ClusterInfo.Name + nginxConfFileExt

	cmd := exec.Command("ansible-playbook", playbookFile, "-i", inventoryFilePath, "-f", "20")
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	// run nginx playbook
	cmd = exec.Command("ansible-playbook", nginxPlaybook, "i", inventoryFilePath, "-f", "20", "-l", "lbnodes", "--extra-vars", "conf_path=", configFilePath)
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// find the all the load balancer cluster for a K8s cluster
// TODO: return slice in stead of an object of pb.LBcluster
func findLBCluster(ClusterName string, lbClusters []*pb.LBcluster) *pb.LBcluster {
	for _, lbCluster := range lbClusters {
		if lbCluster.TargetedK8S == ClusterName {
			return lbCluster
		}
	}
	return nil
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
