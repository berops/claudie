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

type server struct{}

const (
	outputPath              = "services/wireguardian/server/Ansible"
	inventoryFile           = "inventory.ini"
	playbookFile            = "playbook.yml"
	sslPrivateKeyFile       = "private.pem"
	defaultWireguardianPort = 50053
)

func (*server) BuildVPN(_ context.Context, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	log.Info().Msgf("BuildVPN function was invoked with %s", req.Config.Name)
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

		if err := utils.DeleteTmpFiles(outputPath, []string{sslPrivateKeyFile, inventoryFile}); err != nil {
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

	var temp int
	for _, address := range addressesToAssign {
		ip[3] = lastOctets[temp]
		address.Private = ip.String()
		temp++
	}
	// debug message
	for _, address := range addresses {
		fmt.Println(address)
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

	f, err := os.Create(filepath.Join(outputPath, inventoryFile))
	if err != nil {
		return fmt.Errorf("failed to create a inventory file: %v", err)
	}

	if err := tpl.Execute(f, addresses); err != nil {
		return fmt.Errorf("failed to execute template file: %v", err)
	}

	return nil
}

func runAnsible(cluster *pb.Cluster) error {
	if err := utils.CreateKeyFile(cluster.GetPrivateKey(), outputPath, sslPrivateKeyFile); err != nil {
		return err
	}

	if err := os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False"); err != nil {
		return err
	}

	cmd := exec.Command("ansible-playbook", playbookFile, "-i", inventoryFile, "-f", "20", "--private-key", sslPrivateKeyFile)
	cmd.Dir = outputPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

		return errors.New("Interrupt signal")
	})

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("failed to serve: %v", err)
		}
		return nil
	})

	log.Info().Msgf("Stopping Wireguardian: %v", g.Wait())
}
