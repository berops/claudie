package outboundAdapters

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
	"github.com/berops/claudie/services/scheduler/domain/usecases"
)

const (
	defaultHealthcheckPort = 50056
)

type ContextBoxConnector struct {
	Connection *grpc.ClientConn
	usecases   usecases.Usecases
}

func (c *ContextBoxConnector) Connect() {
	connection, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	c.Connection = connection

	log.Info().Msgf("Initiated connection Context-box: %s, waiting for connection to be in state ready", envs.ContextBoxURL)

	// Initialize health probes
	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultHealthcheckPort), c.healthCheck).StartProbes()
}

// healthCheck function is used for querying readiness of the pod running this microservice
func (c *ContextBoxConnector) healthCheck() error {
	res, err := c.usecases.CreateDesiredState(nil)
	if res != nil || err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func (c ContextBoxConnector) GetConfigScheduler(contextBoxGrpcClient pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	return cbox.GetConfigScheduler(contextBoxGrpcClient)
}

func (c ContextBoxConnector) SaveConfigScheduler(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	return cbox.SaveConfigScheduler(contextBoxGrpcClient, &pb.SaveConfigRequest{Config: config})
}

func (c *ContextBoxConnector) Disconnect() {
	utils.CloseClientConnection(c.Connection)
}
