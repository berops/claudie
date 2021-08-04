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
	"strconv"
	"strings"
	"text/template"

	"github.com/Berops/platform/proto/pb"

	"google.golang.org/grpc"
)

type server struct{}

const outputPath string = "services/wireguardian/server/Ansible/"

func (*server) BuildVPN(_ context.Context, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	fmt.Println("BuildVPN function was invoked with", req.Config.Name)
	config := req.GetConfig()

	for _, cluster := range config.GetDesiredState().GetClusters() {
		genPrivAdd(cluster.GetIps(), cluster.GetNetwork())
		genInv(cluster.GetIps())
		runAnsible(cluster)
		deleteTmpFiles()
	}

	return &pb.BuildVPNResponse{Config: config}, nil
}

// genPrivAdd will generate private ip addresses from network parameter
func genPrivAdd(addresses map[string]*pb.Ip, network string) {
	_, ipNet, err := net.ParseCIDR(network)
	var addressesToAssign []*pb.Ip

	// initilize slice of possible last octet
	lastOctets := make([]byte, 255)
	var i byte
	for i = 0; i < 255; i++ {
		lastOctets[i] = i + 1
	}

	if err != nil {
		log.Fatalln(err)
	}
	ip := ipNet.IP
	ip = ip.To4()

	for _, address := range addresses {
		// If address already assigned, skip
		if address.Private != "" {
			lastOctet := strings.Split(address.Private, ".")[3]
			lastOctetInt, _ := strconv.Atoi(lastOctet)
			lastOctets = remove(lastOctets, byte(lastOctetInt))
			continue
		}
		addressesToAssign = append(addressesToAssign, address)
	}

	var temp int = 0
	for _, address := range addressesToAssign {
		ip[3] = lastOctets[temp]
		address.Private = ip.String()
		temp++
	}
}

func remove(slice []byte, value byte) []byte {
	var pos int
	for pos = 0; pos < len(slice); pos++ {
		if slice[pos] == value {
			break
		}
	}
	return append(slice[:pos], slice[pos+1:]...)
}

// genInv will generate ansible inventory file slice of clusters input
func genInv(addresses map[string]*pb.Ip) {
	tpl, err := template.ParseFiles("services/wireguardian/server/inventory.goini")
	if err != nil {
		log.Fatalln("Failed to load template file", err)
	}

	f, err := os.Create(outputPath + "inventory.ini")
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
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalln("Error while executing Ansible", err)
	}
}

func createKeyFile(key string) {
	err := ioutil.WriteFile(outputPath+"private.pem", []byte(key), 0600)
	if err != nil {
		log.Fatalln(err)
	}
}

func deleteTmpFiles() {
	// Delete a private key
	err := os.Remove(outputPath + "private.pem")
	if err != nil {
		log.Fatalln("Error while deleting private.pem file", err)
	}
	// Delete an inventory file
	err = os.Remove(outputPath + "inventory.ini")
	if err != nil {
		log.Fatalln("Error while deleting inventory.ini file", err)
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
