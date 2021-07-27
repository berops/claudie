package main

import (
	"context"
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
		createKeyFile(cluster.GetPrivateKey(), "private.pem")
		genKubeOneConfig(outputPath+"kubeone.tpl", outputPath+"kubeone.yaml", d)
		runKubeOne()
		cluster.Kubeconfig = saveKubeconfig()
		deleteTmpFiles()
	}

	//fmt.Println("Kubeconfig:", string(req.GetCluster().GetKubeconfig()))
	return &pb.BuildClusterResponse{Config: config}, nil
}

func createKeyFile(key string, keyName string) {
	err := ioutil.WriteFile(outputPath+keyName, []byte(key), 0600)
	if err != nil {
		log.Fatalln(err)
	}
}

func genKubeOneConfig(templatePath string, outputPath string, d interface{}) {
	if _, err := os.Stat("kubeone"); os.IsNotExist(err) { //this creates a new file if it doesn't exist
		os.Mkdir("kubeone", os.ModePerm)
	}
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalln("Failed to load the template file", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln("Failed to create the manifest file", err)
	}
	err = tpl.Execute(f, d)
	if err != nil {
		log.Fatalln("Failed to execute the template file", err)
	}
}

func runKubeOne() {
	fmt.Println("Running KubeOne")
	cmd := exec.Command("kubeone", "apply", "-m", "kubeone.yaml", "-y")
	cmd.Dir = outputPath //golang will execute command from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// saveKubeconfig reads kubeconfig from a file and returns it
func saveKubeconfig() string {
	kubeconfig, err := ioutil.ReadFile(outputPath + "cluster-kubeconfig")
	if err != nil {
		log.Fatalln("Error while reading a kubeconfig file", err)
	}
	return string(kubeconfig)
}

func deleteTmpFiles() {
	err := os.Remove(outputPath + "cluster.tar.gz")
	if err != nil {
		log.Fatalln("Error while deleting cluster.tar.gz file", err)
	}
	err = os.Remove(outputPath + "cluster-kubeconfig")
	if err != nil {
		log.Fatalln("Error while deleting cluster-kubeconfig file", err)
	}
	err = os.Remove(outputPath + "kubeone.yaml")
	if err != nil {
		log.Fatalln("Error while deleting kubeone.yaml file", err)
	}
	err = os.Remove(outputPath + "private.pem")
	if err != nil {
		log.Fatalln("Error while deleting private.pem file", err)
	}
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

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Block until a signal is received
	<-ch
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("Closing the listener")
	lis.Close()
	fmt.Println("End of Program")
}
