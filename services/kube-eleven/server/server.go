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

type server struct{}

const outputPath = "services/kube-eleven/server/"

type data struct {
	APIEndpoint string
	Kubernetes  string
	Nodes       []*pb.NodeInfo
}

// formatTemplateData formats data for kubeone template input
func (d *data) formatTemplateData(cluster *pb.Cluster) {
	var controlNodes []*pb.NodeInfo
	var workerNodes []*pb.NodeInfo
	hasAPIEndpoint := false

	for _, nodeInfo := range cluster.GetNodeInfos() {
		if nodeInfo.GetIsControl() == 1 {
			controlNodes = append(controlNodes, nodeInfo)
		} else if nodeInfo.GetIsControl() == 2 {
			hasAPIEndpoint = true
			d.Nodes = append(d.Nodes, nodeInfo) //the Api endpoint must be first in slice
		} else {
			workerNodes = append(workerNodes, nodeInfo)
		}
	}
	if !hasAPIEndpoint {
		controlNodes[0].IsControl = 2
	}
	// if there is something in d.Nodes, it would be rewritten in line 55, therefore this condition
	if len(d.Nodes) > 0 {
		controlNodes = append(d.Nodes, controlNodes...)
	}
	d.Nodes = append(controlNodes, workerNodes...)
	d.Kubernetes = cluster.GetKubernetes()
	d.APIEndpoint = d.Nodes[0].GetPublic()
}

func (*server) BuildCluster(_ context.Context, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	config := req.Config
	log.Info().Msgf("I have received a BuildCluster request with config name: %s", config.GetName())

	for _, cluster := range config.GetDesiredState().GetClusters() {
		var d data
		d.formatTemplateData(cluster)
		// Create a private key file
		if err := utils.CreateKeyFile(cluster.GetPrivateKey(), outputPath, "private.pem"); err != nil {
			return nil, err
		}
		// Create a cluster-kubeconfig file
		kubeconfigFile := filepath.Join(outputPath, "cluster-kubeconfig")
		if err := ioutil.WriteFile(kubeconfigFile, []byte(cluster.GetKubeconfig()), 0600); err != nil {
			return nil, err
		}
		// Generate a kubeOne yaml manifest from a golang template
		templateFile := filepath.Join(outputPath, "kubeone.tpl")
		outputFile := filepath.Join(outputPath, "kubeone.yaml")
		if err := genKubeOneConfig(templateFile, outputFile, d); err != nil {
			return nil, err
		}

		if err := runKubeOne(); err != nil {
			return nil, err
		}

		kc, err := saveKubeconfig()
		if err != nil {
			return nil, err
		}
		cluster.Kubeconfig = kc

		tmpFiles := []string{
			"cluster.tar.gz",
			"cluster-kubeconfig",
			"kubeone.yaml",
			"private.pem",
		}
		if err := utils.DeleteTmpFiles(outputPath, tmpFiles); err != nil {
			return nil, err
		}
	}

	return &pb.BuildClusterResponse{Config: config}, nil
}

func genKubeOneConfig(templatePath string, outputPath string, d interface{}) error {
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create the manifest file: %v", err)
	}

	if err := tpl.Execute(f, d); err != nil {
		return fmt.Errorf("failed to execute the template file: %v", err)
	}

	return nil
}

func runKubeOne() error {
	log.Info().Msg("Running KubeOne")
	cmd := exec.Command("kubeone", "apply", "-m", "kubeone.yaml", "-y")
	cmd.Dir = outputPath //golang will execute command from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// saveKubeconfig reads kubeconfig from a file and returns it
func saveKubeconfig() (string, error) {
	kubeconfigFile := filepath.Join(outputPath, "cluster-kubeconfig")
	kubeconfig, err := ioutil.ReadFile(kubeconfigFile)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig file: %s : %v", kubeconfigFile, err)
	}
	return string(kubeconfig), nil
}

func main() {
	// initialize logger
	utils.InitLog("kube-eleven", "GOLANG_LOG")

	// Set KubeEleven port
	kubeElevenPort := os.Getenv("KUBE_ELEVEN_PORT")
	if kubeElevenPort == "" {
		kubeElevenPort = "50054" // Default value
	}
	kubeElevenAddr := "0.0.0.0:" + kubeElevenPort
	lis, err := net.Listen("tcp", kubeElevenAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", kubeElevenAddr, err)
	}
	log.Info().Msgf("KubeEleven service is listening on %s", kubeElevenAddr)

	s := grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker("50054", "KUBE_ELEVEN_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			<-ch

			signal.Stop(ch)
			s.GracefulStop()

			return errors.New("interrupt signal")
		})
	}
	{
		g.Go(func() error {
			// s.Serve() will create a service goroutine for each connection
			if err := s.Serve(lis); err != nil {
				return fmt.Errorf("failed to serve: %v", err)
			}
			return nil
		})
	}

	log.Info().Msgf("Stopping Kube-Eleven: %s", g.Wait())
}
