package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"text/template"

	"github.com/Berops/platform/proto/pb"

	"google.golang.org/grpc"
)

type server struct{}

func (*server) BuildVPN(_ context.Context, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	fmt.Println("BuildVPN function was invoked with", req.Config.Name)
	config := req.GetConfig()

	for _, cluster := range config.GetDesiredState().GetClusters() {
		genPrivAdd(cluster.GetIps(), cluster.GetNetwork())
		genInv(cluster.GetIps())
		runAnsible(cluster)
	}

	return &pb.BuildVPNResponse{Config: config}, nil
}

// genPrivAdd will generate private ip addresses from network parameter
func genPrivAdd(addresses map[string]*pb.Ip, network string) {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		log.Fatalln(err)
	}
	ip := ipNet.IP
	ip = ip.To4()

	for _, address := range addresses {
		ip[3]++ // check for rollover
		address.Private = ip.String()
	}
}

// genInv will generate ansible inventory file slice of clusters input
func genInv(addresses map[string]*pb.Ip) {
	tpl, err := template.ParseFiles("services/wireguardian/server/inventory.goini")
	if err != nil {
		log.Fatalln("Failed to load template file", err)
	}

	f, err := os.Create("services/wireguardian/server/Ansible/inventory.ini")
	if err != nil {
		log.Fatalln("Failed to create a inventory file", err)
	}

	err = tpl.Execute(f, addresses)
	if err != nil {
		log.Fatalln("Failed to execute template file", err)
	}
}

func runAnsible(cluster *pb.Cluster) {
	createKeyFile(cluster.GetPrivateKey())
	os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False")
	cmd := exec.Command("ansible-playbook", "playbook.yml", "-i", "inventory.ini", "-f", "20", "--private-key", "private.pem")
	cmd.Dir = "services/wireguardian/server/Ansible/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalln("Error while executing Ansible", err)
	}
}

func createKeyFile(key string) {
	err := ioutil.WriteFile("services/wireguardian/server/Ansible/private.pem", []byte(key), 0600)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Set Wireguardian port
	wireguardianPort := os.Getenv("WIREGUARDIAN_PORT")
	if wireguardianPort == "" {
		wireguardianPort = "50053" // Default value
	}

	lis, err := net.Listen("tcp", "0.0.0.0:"+wireguardianPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("Wireguardian service is listening on", "0.0.0.0:"+wireguardianPort)

	// creating a new server
	s := grpc.NewServer()
	pb.RegisterWireguardianServiceServer(s, &server{})

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
