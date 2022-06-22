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
	kuber "github.com/Berops/platform/services/kuber/client"
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

type countsToDelete struct {
	Count uint32
}

type nodesToDelete struct {
	nodes map[string]*countsToDelete // [provider]nodes
}

type etcdNodeInfo struct {
	nodeName string
	nodeHash string
}

func callTerraformer(currentState *pb.Project, desiredState *pb.Project) (*pb.Project, *pb.Project, error) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", urls.TerraformerURL)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for terraformer")
	}()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	log.Info().Msgf("Calling BuildInfrastructure on terraformer")
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		CurrentState: currentState,
		DesiredState: desiredState,
	})
	if err != nil {
		return nil, nil, err
	}

	return res.GetCurrentState(), res.GetDesiredState(), nil
}

func callWireguardian(desiredState, currenState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("wireguardian", urls.WireguardianURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for wireguardian")
	}()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	log.Info().Msgf("Calling RunAnsible on wireguardian")
	res, err := wireguardian.RunAnsible(c, &pb.RunAnsibleRequest{DesiredState: desiredState, CurrentState: currenState})
	if err != nil {
		return nil, err
	}

	return res.GetDesiredState(), nil
}

func callKubeEleven(desiredState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("kubeEleven", urls.KubeElevenURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for kube-eleven")
	}()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	log.Info().Msgf("Calling BuildCluster on kube-eleven")
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}

	return res.GetDesiredState(), nil
}

func callKuber(desiredState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("kuber", urls.KuberURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for kuber")
	}()
	// Creating the client
	c := pb.NewKuberServiceClient(cc)
	log.Info().Msgf("Calling SetUpStorage on kuber")
	resStorage, err := kuber.SetUpStorage(c, &pb.SetUpStorageRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}
	for _, cluster := range desiredState.Clusters {
		log.Info().Msgf("Calling StoreKubeconfig on kuber")
		_, err := kuber.StoreKubeconfig(c, &pb.StoreKubeconfigRequest{Cluster: cluster})
		if err != nil {
			return nil, err
		}
	}
	return resStorage.GetDesiredState(), nil
}

func diff(config *pb.Config) (*pb.Config, bool, map[string]*nodesToDelete) {
	adding, deleting := false, false
	tmpConfig := proto.Clone(config).(*pb.Config)

	type nodeCount struct {
		Count uint32
	}

	type tableKey struct {
		clusterName  string
		nodePoolName string
	}

	var delCounts = make(map[string]*nodesToDelete)

	var tableCurrent = make(map[tableKey]nodeCount)
	for _, cluster := range tmpConfig.GetCurrentState().GetClusters() {
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			tmp := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.ClusterInfo.Name}
			tableCurrent[tmp] = nodeCount{Count: nodePool.Count} // Since a nodepool as only one type of nodes, we'll need only one type of count
		}
	}
	tmpConfigClusters := tmpConfig.GetDesiredState().GetClusters()
	for _, cluster := range tmpConfigClusters {
		tmp := make(map[string]*countsToDelete)
		for _, nodePool := range cluster.ClusterInfo.GetNodePools() {
			var nodesProvider countsToDelete
			key := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.ClusterInfo.Name}

			if _, ok := tableCurrent[key]; ok {
				tmpNodePool := utils.GetNodePoolByName(nodePool.Name, utils.GetClusterByName(cluster.ClusterInfo.Name, tmpConfigClusters).ClusterInfo.GetNodePools())
				if nodePool.Count > tableCurrent[key].Count {
					tmpNodePool.Count = nodePool.Count
					adding = true
				} else if nodePool.Count < tableCurrent[key].Count {
					nodesProvider.Count = tableCurrent[key].Count - nodePool.Count
					tmpNodePool.Count = tableCurrent[key].Count
					deleting = true
				}

				tmp[nodePool.Name] = &nodesProvider
				delete(tableCurrent, key)
			}
		}
		delCounts[cluster.ClusterInfo.Name] = &nodesToDelete{
			nodes: tmp,
		}
	}

	if len(tableCurrent) > 0 {
		for key := range tableCurrent {
			cluster := utils.GetClusterByName(key.clusterName, tmpConfig.DesiredState.Clusters)
			if cluster != nil {
				currentCluster := utils.GetClusterByName(key.clusterName, tmpConfig.CurrentState.Clusters)
				log.Info().Interface("currentCluster", currentCluster)
				cluster.ClusterInfo.NodePools = append(cluster.ClusterInfo.NodePools, utils.GetNodePoolByName(key.nodePoolName, currentCluster.ClusterInfo.GetNodePools()))
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

// function saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}
	return nil
}

// processConfig is function used to carry out task specific to Builder concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient, isTmpConfig bool) (err error) {
	log.Info().Msgf("processConfig received config: %s, is tmpConfig: %t", config.GetName(), isTmpConfig)
	// call Terraformer to build infra
	currentState, desiredState, err := callTerraformer(config.GetCurrentState(), config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Terraformer: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Terraformer: %v", err)
	}
	config.CurrentState = currentState
	config.DesiredState = desiredState
	// call Wireguardian to build VPN
	desiredState, err = callWireguardian(config.GetDesiredState(), config.GetCurrentState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Wireguardian: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Wireguardian: %v", err)
	}
	config.DesiredState = desiredState
	// call Kube-eleven to create K8s clusters
	desiredState, err = callKubeEleven(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in KubeEleven: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in KubeEleven: %v", err)
	}
	config.DesiredState = desiredState

	// call Kuber to set up longhorn
	desiredState, err = callKuber(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Kuber: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Kuber: %v", err)
	}
	config.DesiredState = desiredState

	if !isTmpConfig {
		log.Info().Msgf("Saving the config %s", config.GetName())
		config.CurrentState = config.DesiredState // Update currentState
		err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if err != nil {
			return fmt.Errorf("error while saving the config %s: %v", config.GetName(), err)
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
			var toDelete = make(map[string]*nodesToDelete)
			if len(config.CurrentState.GetClusters()) > 0 {
				tmpConfig, deleting, toDelete = diff(config)
			}
			if tmpConfig != nil {
				log.Info().Msg("Processing a tmpConfig...")
				err := processConfig(tmpConfig, c, true)
				if err != nil {
					return err
				}
				config.CurrentState = tmpConfig.DesiredState
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
	if cc, err := utils.GrpcDialWithInsecure("kubeEleven", urls.KubeElevenURL); err != nil {
		cc.Close()
		return err
	}
	if cc, err := utils.GrpcDialWithInsecure("terraformer", urls.TerraformerURL); err != nil {
		cc.Close()
		return err
	}
	if cc, err := utils.GrpcDialWithInsecure("wireguardian", urls.WireguardianURL); err != nil {
		cc.Close()
		return err
	}
	return nil
}

func deleteNodes(config *pb.Config, toDelete map[string]*nodesToDelete) (*pb.Config, error) {
	for _, cluster := range config.CurrentState.Clusters {
		var nodesToDelete []string
		var etcdToDelete []string
		del := toDelete[cluster.ClusterInfo.Name]
		for _, nodepool := range cluster.ClusterInfo.NodePools {
			for i := len(nodepool.Nodes) - 1; i >= 0; i-- {
				val, ok := del.nodes[nodepool.Name]
				if val.Count > 0 && ok {
					if nodepool.Nodes[i].NodeType > pb.NodeType_worker {
						val.Count--
						nodesToDelete = append(nodesToDelete, nodepool.Nodes[i].GetName())
						etcdToDelete = append(etcdToDelete, nodepool.Nodes[i].GetName())
						log.Info().Msgf("Choosing Master node %s, with public IP %s, private IP %s for deletion\n", nodepool.Nodes[i].GetName(), nodepool.Nodes[i].GetPublic(), nodepool.Nodes[i].GetPrivate())
						continue
					}
					if nodepool.Nodes[i].NodeType == pb.NodeType_worker {
						val.Count--
						nodesToDelete = append(nodesToDelete, nodepool.Nodes[i].GetName())
						log.Info().Msgf("Choosing Worker node %s, with public IP %s, private IP %s for deletion\n", nodepool.Nodes[i].GetName(), nodepool.Nodes[i].GetPublic(), nodepool.Nodes[i].GetPrivate())
						continue
					}
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
			for _, nodepool := range cluster.ClusterInfo.NodePools {
				for idx, node := range nodepool.Nodes {
					if node.GetName() == nodeName {
						nodepool.Count = nodepool.Count - 1
						nodepool.Nodes = append(nodepool.Nodes[:idx], nodepool.Nodes[idx+1:]...)
					}
				}
			}
		}
	}
	return config, nil
}

// deleteNodesByName checks if there is any difference in nodes between a desired state cluster and a running cluster
func deleteNodesByName(cluster *pb.K8Scluster, nodesToDelete []string) error {

	// get node name
	nodesQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") get nodes -n kube-system --no-headers -o custom-columns=\":metadata.name\" ", cluster.GetKubeconfig())
	output, err := exec.Command("bash", "-c", nodesQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of nodes ")
		return err
	}

	// parse list of pods returned
	nodeNames := strings.Split(string(output), "\n")

	//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
	for _, nodeNameSubString := range nodesToDelete {
		nodeName, found := searchNodeNames(nodeNames, nodeNameSubString)
		if found {
			log.Info().Msgf("kubectl drain %s --ignore-daemonsets --delete-local-data", nodeName)
			cmd := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-local-data --kubeconfig <(echo '%s')", nodeName, cluster.GetKubeconfig())
			res, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Error().Msgf("Error while draining node %s : %v", nodeName, err)
				log.Error().Bytes("result", res)
				return err
			}
		} else {
			log.Error().Msgf("Node name that contains \"%s\" no found ", nodeNameSubString)
			return fmt.Errorf("no node with name %s found ", nodeNameSubString)
		}

	}

	//kubectl delete node <node-name>
	for _, nodeNameSubString := range nodesToDelete {
		nodeName, found := searchNodeNames(nodeNames, nodeNameSubString)

		if found {
			log.Info().Msgf("kubectl delete node %s" + nodeName)
			cmd := fmt.Sprintf("kubectl delete node %s --kubeconfig <(echo '%s')", nodeName, cluster.GetKubeconfig())
			_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Error().Msgf("Error while deleting node %s : %v", nodeName, err)
				return err
			}
		} else {
			log.Error().Msgf("Node name that contains \"%s\" no found ", nodeNameSubString)
			return fmt.Errorf("no node with name %s found ", nodeNameSubString)
		}
	}
	return nil
}

func deleteEtcd(cluster *pb.K8Scluster, etcdToDelete []string) error {
	var mainMasterNode *pb.Node
	for _, nodepool := range cluster.ClusterInfo.GetNodePools() {
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint {
				mainMasterNode = node
				break
			}
		}
	}

	if mainMasterNode == nil {
		log.Error().Msg("APIEndpoint node not found")
		return fmt.Errorf("failed to find any node with IsControl value as 2")
	}

	// get etcd pods name
	podsQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") get pods -n kube-system --no-headers -o custom-columns=\":metadata.name\" | grep etcd-%s", cluster.GetKubeconfig(), mainMasterNode.Name)
	output, err := exec.Command("bash", "-c", podsQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of pods with name: etcd-%s", mainMasterNode.Name)
		return err
	}

	// parse list of pods returned
	podNames := strings.Split(string(output), "\n")

	// Execute into the working etcd container and setup client TLS authentication in order to be able to communicate
	// with etcd and get output of all etcd members
	prepCmd := fmt.Sprintf("kubectl --kubeconfig <(echo '%s') -n kube-system exec -i %s -- /bin/sh -c ",
		cluster.GetKubeconfig(), podNames[0])

	exportCmd := "export ETCDCTL_API=3 && " +
		"export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt && " +
		"export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/healthcheck-client.crt && " +
		"export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/healthcheck-client.key"

	cmd := fmt.Sprintf("%s \" %s && etcdctl member list \"", prepCmd, exportCmd)
	output, err = exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Error while executing command %s in a working etcd container: %v", cmd, err)
		log.Error().Msgf("prepCmd was %s", prepCmd)
		return err
	}
	// Convert output into []string, each line of output is a separate string
	etcdStrings := strings.Split(string(output), "\n")
	//delete last entry - empty \n
	if len(etcdStrings) > 0 {
		etcdStrings = etcdStrings[:len(etcdStrings)-1]
	}
	// Example etcdNodesOutput:
	// 3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false
	// 56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false
	var etcdNodeInfos []etcdNodeInfo

	for _, etcdString := range etcdStrings {
		etcdStringTokenized := strings.Split(etcdString, ", ")
		if len(etcdStringTokenized) > 0 {
			temp := etcdNodeInfo{etcdStringTokenized[2] /*name*/, etcdStringTokenized[0] /*hash*/}
			etcdNodeInfos = append(etcdNodeInfos, temp)
		}
	}
	// Remove etcd members that are in etcdToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range etcdToDelete {
		for _, etcdNode := range etcdNodeInfos {
			if nodeName == etcdNode.nodeName {
				log.Info().Msgf("Removing node %s, with hash %s \n", etcdNode.nodeName, etcdNode.nodeHash)
				cmd = fmt.Sprintf("%s \" %s && etcdctl member remove %s \"", prepCmd, exportCmd, etcdNode.nodeHash)
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

func searchNodeNames(nodeNames []string, nodeNameSubString string) (string, bool) {
	// find full nodeName for list of nodes using partial nodename
	for _, nodeName := range nodeNames {
		if strings.Contains(nodeName, nodeNameSubString) {
			return nodeName, true
		}
	}
	return "", false
}

func main() {
	// initialize logger
	utils.InitLog("builder", "GOLANG_LOG")

	// Create connection to Context-box
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	log.Info().Msgf("Dial Context-box: %s", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Msgf("Could not connect to Content-box: %v", err)
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	// Initialize health probes
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
