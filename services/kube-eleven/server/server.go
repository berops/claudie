package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct{}

const outputPath = "services/kube-eleven/server/"

type data struct {
	ApiEndpoint string
	Kubernetes  string
	Nodes       []*pb.Ip
}

// formatTemplateData formats data for kubeone template input
func (d *data) formatTemplateData(cluster *pb.Cluster) {
	for _, ip := range cluster.Ips {
		if ip.GetIsControl() {
			d.Nodes = append(d.Nodes, ip)
		}
	}
	for _, ip := range cluster.Ips {
		if !ip.GetIsControl() {
			d.Nodes = append(d.Nodes, ip)
		}
	}
	d.Kubernetes = cluster.GetKubernetes()
	d.ApiEndpoint = d.Nodes[0].GetPrivate()
}

func (*server) BuildCluster(_ context.Context, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	config := req.Config
	log.Println("I have received a BuildCluster request with config name:", config.GetName())

	for _, cluster := range config.GetDesiredState().GetClusters() {
		var d data
		d.formatTemplateData(cluster)
		if err := createKeyFile(cluster.GetPrivateKey(), "private.pem"); err != nil {
			return nil, err
		}

		if err := genKubeOneConfig(outputPath+"kubeone.tpl", outputPath+"kubeone.yaml", d); err != nil {
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

		if err := deleteTmpFiles(); err != nil {
			return nil, err
		}
	}

	//fmt.Println("Kubeconfig:", string(req.GetCluster().GetKubeconfig()))
	return &pb.BuildClusterResponse{Config: config}, nil
}

func createKeyFile(key string, keyName string) error {
	return ioutil.WriteFile(outputPath+keyName, []byte(key), 0600)
}

func genKubeOneConfig(templatePath string, outputPath string, d interface{}) error {
	if _, err := os.Stat("kubeone"); os.IsNotExist(err) { //this creates a new file if it doesn't exist
		if err := os.Mkdir("kubeone", os.ModePerm); err != nil {
			return err
		}
	}

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
	fmt.Println("Running KubeOne")
	cmd := exec.Command("kubeone", "apply", "-m", "kubeone.yaml", "-y")
	cmd.Dir = outputPath //golang will execute command from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// saveKubeconfig reads kubeconfig from a file and returns it
func saveKubeconfig() (string, error) {
	kubeconfig, err := ioutil.ReadFile(outputPath + "cluster-kubeconfig")
	if err != nil {
		return "", fmt.Errorf("error while reading a kubeconfig file: %v", err)
	}
	return string(kubeconfig), nil
}

func deleteTmpFiles() error {
	if err := os.Remove(outputPath + "cluster.tar.gz"); err != nil {
		return fmt.Errorf("error while deleting cluster.tar.gz file: %v", err)
	}

	if err := os.Remove(outputPath + "cluster-kubeconfig"); err != nil {
		return fmt.Errorf("error while deleting cluster-kubeconfig file: %v", err)
	}

	if err := os.Remove(outputPath + "kubeone.yaml"); err != nil {
		return fmt.Errorf("error while deleting kubeone.yaml file: %v", err)
	}

	if err := os.Remove(outputPath + "private.pem"); err != nil {
		return fmt.Errorf("error while deleting private.pem file: %v", err)
	}

	return nil
}

func main() {
	// If we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Set KubeEleven port
	kubeElevenPort := os.Getenv("KUBE_ELEVEN_PORT")
	if kubeElevenPort == "" {
		kubeElevenPort = "50054" // Default value
	}

	lis, err := net.Listen("tcp", "0.0.0.0:"+kubeElevenPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("KubeEleven service is listening on", "0.0.0.0:"+kubeElevenPort)

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

	log.Println("Stopping Kube-Eleven: ", g.Wait())
}
