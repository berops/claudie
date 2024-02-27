package usecases

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils"
	"github.com/berops/claudie/services/kuber/server/domain/utils/secret"
)

// StoreClusterMetadata constructs ClusterMetadata for the given K8s cluster, creates a Kubernetes
// secret out of that and stores that secret in the Claudie management cluster.
func (u *Usecases) StoreClusterMetadata(ctx context.Context, request *pb.StoreClusterMetadataRequest) (*pb.StoreClusterMetadataResponse, error) {
	logger := cutils.CreateLoggerWithClusterName(cutils.GetClusterID(request.Cluster.ClusterInfo))

	md := ClusterMetadata{
		PrivateKey: request.GetCluster().GetClusterInfo().GetPrivateKey(),
	}

	dp := DynamicNodepool{NodeIps: make(map[string]IPPair)}
	sp := StaticNodepool{NodeInfo: make(map[string]StaticNodeInfo)}
	for _, pool := range request.GetCluster().GetClusterInfo().GetNodePools() {
		if np := pool.GetDynamicNodePool(); np != nil {
			for _, node := range pool.GetNodes() {
				dp.NodeIps[node.GetName()] = IPPair{
					PublicIP:  net.ParseIP(node.GetPublic()),
					PrivateIP: net.ParseIP(node.GetPrivate()),
				}
			}
		} else if np := pool.GetStaticNodePool(); np != nil {
			for _, node := range pool.GetNodes() {
				sp.NodeInfo[node.GetName()] = StaticNodeInfo{
					PrivateKey: np.NodeKeys[node.Public],
					Endpoint:   node.GetPublic()}
			}
		}
	}
	md.DynamicNodepools = dp
	md.StaticNodepools = sp

	lbdp := make(map[string]DynamicLoadBalancerNodePools)
	lbst := make(map[string]StaticLoadBalancerNodePools)

	for _, lb := range request.GetLoadbalancers() {
		for _, pool := range lb.GetClusterInfo().GetNodePools() {
			if np := pool.GetDynamicNodePool(); np != nil {
				lbdp[lb.GetClusterInfo().GetName()] = DynamicLoadBalancerNodePools{
					NodeIps:    make(map[string]IPPair),
					PrivateKey: lb.GetClusterInfo().GetPrivateKey(),
				}

				for _, node := range pool.GetNodes() {
					lbdp[lb.GetClusterInfo().GetName()].NodeIps[node.GetName()] = IPPair{
						PublicIP:  net.ParseIP(node.GetPublic()),
						PrivateIP: net.ParseIP(node.GetPrivate()),
					}
				}
			} else if np := pool.GetStaticNodePool(); np != nil {
				lbst[lb.GetClusterInfo().GetName()] = StaticLoadBalancerNodePools{NodeInfo: make(map[string]StaticNodeInfo)}

				for _, node := range pool.GetNodes() {
					lbst[lb.GetClusterInfo().GetName()].NodeInfo[node.GetName()] = StaticNodeInfo{
						PrivateKey: np.NodeKeys[node.Public],
						Endpoint:   node.GetPublic()}
				}
			}
		}
	}

	md.DynamicLoadBalancerNodePools = lbdp
	md.StaticLoadBalancerNodePools = lbst

	b, err := json.Marshal(md)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s cluster metadata: %w", request.GetCluster().GetClusterInfo().GetName(), err)
	}

	// local deployment - print metadata
	if envs.Namespace == "" {
		// NOTE: DEBUG print
		// var buffer bytes.Buffer
		// for node, ips := range md.NodeIps {
		// 	buffer.WriteString(fmt.Sprintf("%s: %v \t| %v \n", node, ips.PublicIP, ips.PrivateIP))
		// }
		// buffer.WriteString(fmt.Sprintf("%s\n", md.PrivateKey))
		// log.Info().Msgf("Cluster metadata from cluster %s \n%s", req.GetCluster().ClusterInfo.Name, buffer.String())
		return &pb.StoreClusterMetadataResponse{}, nil
	}
	logger.Info().Msgf("Storing cluster metadata")

	clusterID := cutils.GetClusterID(request.GetCluster().ClusterInfo)
	clusterDir := filepath.Join(outputDir, clusterID)
	sec := secret.New(clusterDir, secret.NewYaml(
		utils.GetSecretMetadata(request.Cluster.ClusterInfo, request.ProjectName, utils.MetadataSecret),
		map[string]string{"metadata": base64.StdEncoding.EncodeToString(b)},
	))

	if err := sec.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store cluster metadata")
		return nil, fmt.Errorf("error while creating cluster metadata secret for %s", request.Cluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Cluster metadata was successfully stored")
	return &pb.StoreClusterMetadataResponse{}, nil
}
