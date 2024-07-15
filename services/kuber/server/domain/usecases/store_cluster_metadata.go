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

	dp := make(map[string]DynamicNodepool)
	sp := make(map[string]StaticNodepool)

	for _, pool := range request.GetCluster().GetClusterInfo().GetNodePools() {
		if np := pool.GetDynamicNodePool(); np != nil {
			dp[pool.Name] = DynamicNodepool{
				NodeIps:    make(map[string]IPPair),
				PrivateKey: np.PrivateKey,
			}
			for _, node := range pool.GetNodes() {
				dp[pool.Name].NodeIps[node.GetName()] = IPPair{
					PublicIP:  net.ParseIP(node.GetPublic()),
					PrivateIP: net.ParseIP(node.GetPrivate()),
				}
			}
		} else if np := pool.GetStaticNodePool(); np != nil {
			for _, node := range pool.GetNodes() {
				sp[pool.Name].NodeInfo[node.GetName()] = StaticNodeInfo{
					PrivateKey: np.NodeKeys[node.Public],
					Endpoint:   node.GetPublic()}
			}
		}
	}

	lbdp := make(map[string]map[string]DynamicNodepool)
	lbst := make(map[string]map[string]StaticNodepool)

	for _, lb := range request.GetLoadbalancers() {
		lbdp[lb.GetClusterInfo().GetName()] = make(map[string]DynamicNodepool)
		for _, pool := range lb.GetClusterInfo().GetNodePools() {
			if np := pool.GetDynamicNodePool(); np != nil {
				lbdp[lb.GetClusterInfo().GetName()][pool.Name] = DynamicNodepool{
					NodeIps:    make(map[string]IPPair),
					PrivateKey: np.PrivateKey,
				}
				for _, node := range pool.GetNodes() {
					lbdp[lb.GetClusterInfo().GetName()][pool.Name].NodeIps[node.GetName()] = IPPair{
						PublicIP:  net.ParseIP(node.GetPublic()),
						PrivateIP: net.ParseIP(node.GetPrivate()),
					}
				}
			} else if np := pool.GetStaticNodePool(); np != nil {
				lbst[lb.GetClusterInfo().GetName()][pool.Name] = StaticNodepool{
					NodeInfo: make(map[string]StaticNodeInfo),
				}

				for _, node := range pool.GetNodes() {
					lbst[lb.GetClusterInfo().GetName()][pool.Name].NodeInfo[node.GetName()] = StaticNodeInfo{
						PrivateKey: np.NodeKeys[node.Public],
						Endpoint:   node.GetPublic()}
				}
			}
		}
	}

	md := ClusterMetadata{
		DynamicNodepools:             dp,
		StaticNodepools:              sp,
		DynamicLoadBalancerNodePools: lbdp,
		StaticLoadBalancerNodePools:  lbst,
	}

	b, err := json.Marshal(md)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s cluster metadata: %w", request.GetCluster().GetClusterInfo().GetName(), err)
	}

	// local deployment
	if envs.Namespace == "" {
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
