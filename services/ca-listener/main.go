package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"net"
	"strings"

	"github.com/Berops/claudie/internal/utils"
	claudie_provider "github.com/Berops/claudie/services/ca-listener/claudie-provider"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

// MultiStringFlag is a flag for passing multiple parameters using same flag
type MultiStringFlag []string

// String returns string representation of the node groups.
func (flag *MultiStringFlag) String() string {
	return "[" + strings.Join(*flag, " ") + "]"
}

// Set adds a new configuration.
func (flag *MultiStringFlag) Set(value string) error {
	*flag = append(*flag, value)
	return nil
}

func multiStringFlag(name string, usage string) *MultiStringFlag {
	value := new(MultiStringFlag)
	flag.Var(value, name, usage)
	return value
}

var (
	// Flags needed by the external grpc provider service.
	address = flag.String("address", ":8086", "The address to expose the grpc service.")
	keyCert = flag.String("key-cert", "", "The path to the certificate key file. Empty string for insecure communication.")
	cert    = flag.String("cert", "", "The path to the certificate file. Empty string for insecure communication.")
	cacert  = flag.String("ca-cert", "", "The path to the ca certificate file. Empty string for insecure communication.")

	// Flags needed by the specific cloud provider.
	projectName = flag.String("project-name", "", "Autoscaled config name")
	clusterName = flag.String("cluster-name", "", "Autoscaled cluster name")
)

func main() {
	flag.Parse()
	// Exit if flags are not defined
	if *projectName == "" || *clusterName == "" {
		log.Fatal().Msgf("Flags --project-name and --project-name must be specified")
	}

	utils.InitLog("claudie-ca-provider")
	var server *grpc.Server

	// Check for TLS config.
	var serverOpt grpc.ServerOption
	if *keyCert == "" || *cert == "" || *cacert == "" {
		log.Info().Msgf("no cert specified, using insecure")
		server = grpc.NewServer()
	} else {
		certificate, err := tls.LoadX509KeyPair(*cert, *keyCert)
		if err != nil {
			log.Fatal().Msgf("failed to read certificate files: %s", err)
		}
		certPool := x509.NewCertPool()
		bs, err := ioutil.ReadFile(*cacert)
		if err != nil {
			log.Fatal().Msgf("failed to read client ca cert: %s", err)
		}
		ok := certPool.AppendCertsFromPEM(bs)
		if !ok {
			log.Fatal().Msgf("failed to append client certs")
		}
		transportCreds := credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		})
		serverOpt = grpc.Creds(transportCreds)
		server = grpc.NewServer(serverOpt)
	}

	// Listen
	lis, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal().Msgf("failed to listen: %s", err)
	}

	// Serve
	srv := claudie_provider.NewClaudieCloudProvider(projectName, clusterName)
	protos.RegisterCloudProviderServer(server, srv)
	log.Info().Msgf("Server ready at: %s\n", *address)
	if err := server.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}

}
