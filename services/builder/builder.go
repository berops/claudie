package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/Berops/platform/healthcheck"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
	"github.com/Berops/platform/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"google.golang.org/protobuf/proto"
)

const defaultBuilderPort = 50051

type nodesToDelete struct {
	masterCount uint32
	workerCount uint32
}

type countsToDelete struct {
	nodes map[string]*nodesToDelete // [provider]nodes
}

func callTerraformer(currentState *pb.Project, desiredState *pb.Project) (*pb.Project, *pb.Project, error) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", urls.TerraformerURL)
	if err != nil {
		return nil, nil, err
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		CurrentState: currentState,
		DesiredState: desiredState,
	})
	if err != nil {
		return currentState, desiredState, err
	}

	return res.GetCurrentState(), res.GetDesiredState(), nil
}

func callWireguardian(config *pb.Config) (*pb.Config, error) {
	cc, err := utils.GrpcDialWithInsecure("wireguardian", urls.WireguardianURL)
	if err != nil {
		return nil, err
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	res, err := wireguardian.BuildVPN(c, &pb.BuildVPNRequest{Config: config})
	if err != nil {
		return res.GetConfig(), err
	}

	return res.GetConfig(), nil
}

func callKubeEleven(config *pb.Config) (*pb.Config, error) {
	cc, err := utils.GrpcDialWithInsecure("kubeEleven", urls.KubeElevenURL)
	if err != nil {
		return nil, err
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{Config: config})
	if err != nil {
		return res.GetConfig(), err
	}

	return res.GetConfig(), nil
}

func diff(config *pb.Config) (*pb.Config, bool, map[string]*countsToDelete) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)

	type nodeCount struct {
		masterCount uint32
		workerCount uint32
	}

	type tableKey struct {
		clusterName  string
		nodePoolName string
	}

	var delCounts = make(map[string]*countsToDelete)

	var tableCurrent = make(map[tableKey]nodeCount)
	for _, cluster := range tmpConfig.GetCurrentState().GetClusters() {
		for _, nodePool := range cluster.GetNodePools() {
			tmp := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.Name}
			tableCurrent[tmp] = nodeCount{nodePool.Master.Count, nodePool.Worker.Count}
		}
	}
	tmpConfigClusters := tmpConfig.GetDesiredState().GetClusters()
	for _, cluster := range tmpConfigClusters {
		tmp := make(map[string]*nodesToDelete)
		for _, nodePool := range cluster.GetNodePools() {
			var nodesProvider nodesToDelete
			key := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.Name}

			if _, ok := tableCurrent[key]; ok {
				tmpNodePool := getNodePoolByName(nodePool.Name, utils.GetClusterByName(cluster.Name, tmpConfigClusters).GetNodePools())
				if nodePool.Master.Count > tableCurrent[key].masterCount {
					tmpNodePool.Master.Count = nodePool.Master.Count
					adding = true
				} else if nodePool.Master.Count < tableCurrent[key].masterCount {
					nodesProvider.masterCount = tableCurrent[key].masterCount - nodePool.Master.Count
					tmpNodePool.Master.Count = tableCurrent[key].masterCount
					deleting = true
				}
				if nodePool.Worker.Count > tableCurrent[key].workerCount {
					tmpNodePool.Worker.Count = nodePool.Worker.Count
					adding = true
				} else if nodePool.Worker.Count < tableCurrent[key].workerCount {
					nodesProvider.workerCount = tableCurrent[key].workerCount - nodePool.Worker.Count
					tmpNodePool.Worker.Count = tableCurrent[key].workerCount
					deleting = true
				}
				tmp[nodePool.Provider.Name] = &nodesProvider
				delete(tableCurrent, key)
			}
		}
		delCounts[cluster.Name] = &countsToDelete{
			nodes: tmp,
		}
	}

	if len(tableCurrent) > 0 {
		for key := range tableCurrent {
			cluster := utils.GetClusterByName(key.clusterName, tmpConfig.DesiredState.Clusters)
			if cluster != nil {
				currentCluster := utils.GetClusterByName(key.clusterName, tmpConfig.CurrentState.Clusters)
				log.Info().Interface("currentCluster", currentCluster)
				cluster.NodePools = append(cluster.NodePools, getNodePoolByName(key.nodePoolName, currentCluster.GetNodePools()))
				deleting = true
			}
		}
	}

	switch {
	case adding && deleting:
		return tmpConfig, deleting, delCounts
	case deleting:
		return nil, deleting, delCounts
	default:
		return nil, deleting, nil
	}
}

// getNodePoolByName will return first Nodepool that will have same name as specified in parameters
// If no name is found, return nil
func getNodePoolByName(nodePoolName string, nodePools []*pb.NodePool) *pb.NodePool {
	if nodePoolName == "" {
		return nil
	}
	for _, np := range nodePools {
		if np.Name == nodePoolName {
			return np
		}
	}
	return nil
}

// processConfig is function used to carry out task specific to Builder concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient, tmp bool) (err error) {
	log.Info().Msgf("processConfig received config: %s", config.GetName())
	currentState, desiredState, err := callTerraformer(config.GetCurrentState(), config.GetDesiredState())
	if err != nil && config != nil {
		config.CurrentState = config.DesiredState // Update currentState
		// save error message to config
		config.ErrorMessage = err.Error()
		errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if errSave != nil {
			return fmt.Errorf("error while saving the config: %v", err)
		}
		return fmt.Errorf("error in Terraformer: %v", err)
	}
	config.CurrentState = currentState
	config.DesiredState = desiredState

	config, err = callWireguardian(config)
	if err != nil && config != nil {
		config.CurrentState = config.DesiredState // Update currentState
		// save error message to config
		config.ErrorMessage = err.Error()
		errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if errSave != nil {
			return fmt.Errorf("error while saving the config: %v", err)
		}
		return fmt.Errorf("error in Wireguardian: %v", err)
	}

	config, err = callKubeEleven(config)
	if err != nil && config != nil {
		config.CurrentState = config.DesiredState // Update currentState
		// save error message to config
		config.ErrorMessage = err.Error()
		errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if errSave != nil {
			return fmt.Errorf("error while saving the config: %v", err)
		}
		return fmt.Errorf("error in KubeEleven: %v", err)
	}

	if !tmp && config != nil {
		config.CurrentState = config.DesiredState // Update currentState
		errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if errSave != nil {
			return fmt.Errorf("error while saving the config: %v", err)
		}
	}

	return nil
}

func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		res, err := cbox.GetConfigBuilder(c) // Get a new config
		if err != nil {
			return fmt.Errorf("error while getting config from the Builder: %v", err)
		}

		config := res.GetConfig()
		if config != nil {
			var tmpConfig *pb.Config
			var deleting bool
			var toDelete = make(map[string]*countsToDelete)
			if len(config.CurrentState.GetClusters()) > 0 {
				tmpConfig, deleting, toDelete = diff(config)
			}
			if tmpConfig != nil {
				log.Info().Msg("Processing a tmpConfig...")
				err := processConfig(tmpConfig, c, true)
				if err != nil {
					return err
				}
			}
			if deleting {
				log.Info().Msg("Deleting nodes...")
				config, err = deleteNodes(config, toDelete)
				if err != nil {
					return err
				}
			}

			log.Info().Msgf("Processing config %s", config.Name)
			go func() {
				err := processConfig(config, c, false)
				if err != nil {
					log.Error().Err(err)
				}
			}()
		}
		return nil
	}
}

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck() error {
	//Check if Builder can connect to Terraformer/Wireguardian/Kube-eleven
	//Connection to these services are crucial for Builder, without them, the builder is NOT Ready
	if _, err := utils.GrpcDialWithInsecure("kubeEleven", urls.KubeElevenURL); err != nil {
		return err
	}
	if _, err := utils.GrpcDialWithInsecure("terraformer", urls.TerraformerURL); err != nil {
		return err
	}
	if _, err := utils.GrpcDialWithInsecure("wireguardian", urls.WireguardianURL); err != nil {
		return err
	}
	return nil
}

func deleteNodes(config *pb.Config, toDelete map[string]*countsToDelete) (*pb.Config, error) {
	for _, cluster := range config.CurrentState.Clusters {
		var nodesToDelete []string
		var etcdToDelete []string
		del := toDelete[cluster.Name]
		for i := len(cluster.NodeInfos) - 1; i >= 0; i-- {
			val, ok := del.nodes[cluster.NodeInfos[i].Provider]
			if ok {
				if val.masterCount > 0 && cluster.NodeInfos[i].IsControl > 0 {
					val.masterCount--
					nodesToDelete = append(nodesToDelete, cluster.NodeInfos[i].GetNodeName())
					etcdToDelete = append(etcdToDelete, cluster.NodeInfos[i].GetNodeName())
					continue
				}
				if val.workerCount > 0 && cluster.NodeInfos[i].IsControl == 0 {
					val.workerCount--
					nodesToDelete = append(nodesToDelete, cluster.NodeInfos[i].NodeName)
					continue
				}
			}
		}

		// Delete nodes from an etcd
		if len(etcdToDelete) > 0 {
			err := deleteEtcd(cluster, etcdToDelete)
			if err != nil {
				return nil, err
			}
		}
		// Delete nodes from a cluster
		err := deleteNodesByName(cluster, nodesToDelete)
		if err != nil {
			return nil, err
		}

		// Delete nodes from a current state Ips map
		for _, nodeName := range nodesToDelete {
			for _, val := range cluster.NodeInfos {
				if val.GetNodeName() == nodeName {
					nodepool := getNodePoolByName(val.NodepoolName, cluster.NodePools)
					if val.IsControl > 1 {
						// decrement master count
						nodepool.Master.Count = nodepool.GetMaster().GetCount() - 1
					} else {
						// decrement worker count
						nodepool.Worker.Count = nodepool.GetWorker().GetCount() - 1
					}
				}
			}
			cluster.NodeInfos = remove(cluster.NodeInfos, nodeName)
		}
	}
	return config, nil
}

func remove(slice []*pb.NodeInfo, value string) []*pb.NodeInfo {
	for idx, v := range slice {
		if v.GetNodeName() == value {
			return append(slice[:idx], slice[idx+1:]...)
		}
	}
	return slice
}

// deleteNodesByName checks if there is any difference in nodes between a desired state cluster and a running cluster
func deleteNodesByName(cluster *pb.Cluster, nodesToDelete []string) error {
	//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
	for _, node := range nodesToDelete {
		log.Info().Msgf("kubectl drain %s --ignore-daemonsets --delete-local-data", node)
		cmd := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-local-data --kubeconfig <(echo '%s')", node, cluster.GetKubeconfig())
		res, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			log.Error().Msgf("Error while draining node %s : %v", node, err)
			log.Error().Bytes("result", res)
			return err
		}
	}

	//kubectl delete node <node-name>
	for _, node := range nodesToDelete {
		log.Info().Msgf("kubectl delete node %s" + node)
		cmd := fmt.Sprintf("kubectl delete node %s --kubeconfig <(echo '%s')", node, cluster.GetKubeconfig())
		_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			log.Error().Msgf("Error while deleting node %s : %v", node, err)
			return err
		}
	}
	return nil
}

func deleteEtcd(cluster *pb.Cluster, etcdToDelete []string) error {
	mainMaster := cluster.GetNodeInfos()[0]

	// Execute into the working etcd container and setup client TLS authentication in order to be able to communicate
	// with etcd and get output of all etcd members
	prepCmd := fmt.Sprintf("kubectl --kubeconfig <(echo '%s') -n kube-system exec -i etcd-%s -- /bin/sh -c ",
		cluster.GetKubeconfig(), mainMaster.GetNodeName())
	exportCmd := "export ETCDCTL_API=3 && " +
		"export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt && " +
		"export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/healthcheck-client.crt && " +
		"export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/healthcheck-client.key"
	cmd := fmt.Sprintf("%s \" %s && etcdctl member list \"", prepCmd, exportCmd)
	output, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Error while executing command %s in a working etcd container: %v", cmd, err)
		log.Error().Msgf("prepCmd was %s", prepCmd)
		return err
	}
	// Convert output into []string, each line of output is a separate string
	etcdStrings := strings.Fields(string(output))
	// Example etcdNodesOutput:
	// 3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false
	// 56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false
	// Trim "," from every string
	var etcdStringsTrimmed []string
	for _, s := range etcdStrings {
		s = strings.TrimSuffix(s, ",")
		etcdStringsTrimmed = append(etcdStringsTrimmed, s)
	}
	// Remove etcd members that are in etcdToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range etcdToDelete {
		for i, s := range etcdStringsTrimmed {
			if nodeName == s {
				cmd = fmt.Sprintf("%s \" %s && etcdctl member remove %s \"", prepCmd, exportCmd, etcdStringsTrimmed[i-2])
				_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
				if err != nil {
					log.Error().Msgf("Error while etcdctl member remove: %v", err)
					log.Error().Msgf("prepCmd was %s", prepCmd)
					return err
				}
			}
		}
	}

	return nil
}

func main() {
	// initialize logger
	utils.InitLog("builder", "GOLANG_LOG")

	// Create connection to Context-box
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Msgf("Could not connect to Content-box: %v", err)
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultBuilderPort), healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(ctx, 5*time.Second, configProcessor(c), worker.ErrorLogger)

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch
		return errors.New("builder interrupt signal")
	})

	g.Go(func() error {
		w.Run()
		return nil
	})

	log.Info().Msgf("Stopping Builder: %v", g.Wait())
}
