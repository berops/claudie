package main

import (
	"context"
	"errors"
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

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct{}

const outputPath string = "services/wireguardian/server/Ansible/"

func (*server) BuildVPN(_ context.Context, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	fmt.Println("BuildVPN function was invoked with", req.Config.Name)
	config := req.GetConfig()

	for _, cluster := range config.GetDesiredState().GetClusters() {
		if err := genPrivAdd(cluster.GetNodeInfos(), cluster.GetNetwork()); err != nil {
			return nil, err
		}

		if err := genInv(cluster.GetNodeInfos()); err != nil {
			return nil, err
		}

		if err := runAnsible(cluster); err != nil {
			return nil, err
		}

		if err := deleteTmpFiles(); err != nil {
			return nil, err
		}
	}

	return &pb.BuildVPNResponse{Config: config}, nil
}

// genPrivAdd will generate private ip addresses from network parameter
func genPrivAdd(addresses []*pb.NodeInfo, network string) error {
	_, ipNet, err := net.ParseCIDR(network)
	var addressesToAssign []*pb.NodeInfo

	// initilize slice of possible last octet
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
	return nil
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
func genInv(addresses []*pb.NodeInfo) error {
	tpl, err := template.ParseFiles("services/wireguardian/server/inventory.goini")
	if err != nil {
		return fmt.Errorf("failed to load template file: %v", err)
	}

	f, err := os.Create(outputPath + "inventory.ini")
	if err != nil {
		return fmt.Errorf("failed to create a inventory file: %v", err)
	}

	if err := tpl.Execute(f, addresses); err != nil {
		return fmt.Errorf("failed to execute template file: %v", err)
	}

	return nil
}

func runAnsible(cluster *pb.Cluster) error {
	if err := createKeyFile(cluster.GetPrivateKey()); err != nil {
		return err
	}

	if err := os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False"); err != nil {
		return err
	}

	cmd := exec.Command("ansible-playbook", "playbook.yml", "-i", "inventory.ini", "-f", "20", "--private-key", "private.pem")
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createKeyFile(key string) error {
	return ioutil.WriteFile(outputPath+"private.pem", []byte(key), 0600)
}

func deleteTmpFiles() error {
	// Delete a private key
	if err := os.Remove(outputPath + "private.pem"); err != nil {
		return fmt.Errorf("error while deleting private.pem file: %v", err)
	}
	// Delete an inventory file
	if err := os.Remove(outputPath + "inventory.ini"); err != nil {
		return fmt.Errorf("error while deleting inventory.ini file: %v", err)
	}

	return nil
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

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker("50053", "WIREGUARDIAN_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			defer signal.Stop(ch)
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

	log.Println("Stopping Wireguardian: ", g.Wait())
}
