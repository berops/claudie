package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedKubeElevenServiceServer
}

const (
	outputPath            = "services/kube-eleven/server/" // path to the output directory
	defaultKubeElevenPort = 50054                          // default port for kube-eleven
)

type data struct {
	APIEndpoint string
	Kubernetes  string
	Nodes       []*pb.Node
}

// formatTemplateData formats data for kubeone template input
func (d *data) formatTemplateData(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster) {
	var controlNodes []*pb.Node
	var workerNodes []*pb.Node
	hasAPIEndpoint := false

	// Get the API endpoint. If it is not set, use the first control node
	for _, lbCluster := range lbClusters {
		if lbCluster.TargetedK8S == cluster.ClusterInfo.Name {
			//check if the lb is api-lb
			for _, role := range lbCluster.Roles {
				if role.RoleType == pb.RoleType_ApiServer {
					hasAPIEndpoint = true
					d.APIEndpoint = lbCluster.Dns.Endpoint
				}
			}
		}
	}
	for _, Nodepool := range cluster.ClusterInfo.GetNodePools() {
		for _, Node := range Nodepool.Nodes {
			if Node.GetNodeType() == pb.NodeType_master {
				controlNodes = append(controlNodes, Node)
			} else if Node.GetNodeType() == pb.NodeType_apiEndpoint {
				hasAPIEndpoint = true
				d.Nodes = append(d.Nodes, Node) //the Api endpoint must be first in slice
			} else {
				workerNodes = append(workerNodes, Node)
			}
		}
	}
	if !hasAPIEndpoint {
		controlNodes[0].NodeType = pb.NodeType_apiEndpoint
	}
	// if there is something in d.Nodes, it would be rewritten in line 55, therefore this condition
	if len(d.Nodes) > 0 {
		controlNodes = append(d.Nodes, controlNodes...)
	}
	d.Nodes = append(controlNodes, workerNodes...)
	d.Kubernetes = cluster.GetKubernetes()
	if d.APIEndpoint == "" {
		d.APIEndpoint = d.Nodes[0].GetPublic()
	}
}

// BuildCluster builds all cluster defined in the desired state
func (*server) BuildCluster(_ context.Context, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	desiredState := req.GetDesiredState()
	log.Info().Msgf("I have received a BuildCluster request with Project name: %s", desiredState.GetName())

	var errGroup errgroup.Group

	// Build all clusters
	for _, cluster := range desiredState.GetClusters() {
		func(cluster *pb.K8Scluster) {
			errGroup.Go(func() error {
				err := buildClusterAsync(cluster, desiredState.LoadBalancerClusters)
				if err != nil {
					log.Error().Msgf("error encountered in KubeEleven - BuildCluster: %v", err)
					return err
				}
				return nil
			})
		}(cluster)
	}
	err := errGroup.Wait()
	if err != nil {
		return &pb.BuildClusterResponse{DesiredState: desiredState, ErrorMessage: err.Error()}, err
	}
	return &pb.BuildClusterResponse{DesiredState: desiredState, ErrorMessage: ""}, nil
}

// buildClusterAsync builds a kubeone cluster
// It is executed in a goroutine
func buildClusterAsync(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster) error {
	var d data
	d.formatTemplateData(cluster, lbClusters)

	// Create a directory for the cluster
	clusterOutputPath := filepath.Join(outputPath, cluster.ClusterInfo.GetName()+"-"+cluster.ClusterInfo.GetHash())

	// Create a directory for the cluster
	if _, err := os.Stat(clusterOutputPath); os.IsNotExist(err) {
		if err := os.MkdirAll(clusterOutputPath, os.ModePerm); err != nil {
			log.Info().Msgf("error while creating dir %s: %v", clusterOutputPath, err)
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	// Create a private key file
	if err := utils.CreateKeyFile(cluster.ClusterInfo.GetPrivateKey(), clusterOutputPath, "private.pem"); err != nil {
		log.Info().Msgf("error while key file: %v", err)
		return err
	}

	// Create a cluster-kubeconfig file
	kubeconfigFilePath := filepath.Join(clusterOutputPath, "cluster-kubeconfig")
	if err := ioutil.WriteFile(kubeconfigFilePath, []byte(cluster.GetKubeconfig()), 0600); err != nil {
		log.Info().Msgf("error while writing cluster-kubeconfig in %s: %v", clusterOutputPath, err)
		return err
	}

	// Generate a kubeOne yaml manifest from a golang template
	templateFilePath := filepath.Join(outputPath, "kubeone.tpl")
	manifestFilePath := filepath.Join(clusterOutputPath, "kubeone.yaml")
	if err := genKubeOneConfig(templateFilePath, manifestFilePath, d); err != nil {
		log.Info().Msgf("error while generating kubeone.yaml in %s: %v", clusterOutputPath, err)
		return err
	}

	// Run kubeone
	if err := runKubeOne(clusterOutputPath); err != nil {
		log.Info().Msgf("error while running kubeone in %s: %v", clusterOutputPath, err)
		return err
	}

	// Save generated kubeconfig file to cluster config
	kc, err := readKubeconfig(kubeconfigFilePath)
	if err != nil {
		log.Info().Msgf("error while reading cluster-config in %s: %v", clusterOutputPath, err)
		return err
	}
	//check if kubeconfig is not empty
	if len(kc) > 0 {
		cluster.Kubeconfig = kc
	}

	// Clean up
	if err := os.RemoveAll(clusterOutputPath); err != nil {
		log.Info().Msgf("error while removing files from %s: %v", clusterOutputPath, err)
		return err
	}

	return nil
}

// genKubeOneConfig generates a kubeone yaml manifest from a golang template
func genKubeOneConfig(templateFilePath string, manifestFilePath string, d interface{}) error {
	// Read the template file
	tpl, err := template.ParseFiles(templateFilePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}

	// Create a file for the manifest
	f, err := os.Create(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to create the manifest file: %v", err)
	}

	// Execute the template and write to the manifest file
	if err := tpl.Execute(f, d); err != nil {
		// Error is probably because the template is not valid
		return fmt.Errorf("failed to execute the template file: %v", err)
	}

	return nil
}

// runKubeOne runs kubeone with the generated manifest
func runKubeOne(path string) error {
	log.Info().Msgf("Running KubeOne in %s dir", path)
	cmd := exec.Command("kubeone", "apply", "-m", "kubeone.yaml", "-y")
	cmd.Dir = path // golang will execute command from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// readKubeconfig reads kubeconfig from a file and returns it
func readKubeconfig(kubeconfigFile string) (string, error) {
	kubeconfig, err := ioutil.ReadFile(kubeconfigFile)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig file %s : %v", kubeconfigFile, err)
	}
	return string(kubeconfig), nil
}

func main() {
	// initialize logger
	utils.InitLog("kube-eleven", "GOLANG_LOG")

	// Set KubeEleven port
	kubeElevenPort := utils.GetenvOr("KUBE_ELEVEN_PORT", fmt.Sprint(defaultKubeElevenPort))
	kubeElevenAddr := net.JoinHostPort("0.0.0.0", kubeElevenPort)
	lis, err := net.Listen("tcp", kubeElevenAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", kubeElevenAddr, err)
	}
	log.Info().Msgf("KubeEleven service is listening on %s", kubeElevenAddr)

	s := grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(kubeElevenPort, "KUBE_ELEVEN_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch

		signal.Stop(ch)
		s.GracefulStop()

		return errors.New("KubeEleven interrupt signal")
	})

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("KubeEleven failed to serve: %v", err)
		}
		return nil
	})

	log.Info().Msgf("Stopping KubeEleven: %s", g.Wait())
}
