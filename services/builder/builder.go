package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/Berops/platform/healthcheck"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"
	"github.com/Berops/platform/utils"
	"github.com/Berops/platform/worker"
	"golang.org/x/sync/errgroup"

	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"github.com/Berops/platform/urls"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type nodesToDelete struct {
	masterCount uint32
	workerCount uint32
}

type countsToDelete struct {
	nodes map[string]*nodesToDelete //[provider]nodes
}

func callTerraformer(config *pb.Config) (*pb.Config, error) {
	// Create connection to Terraformer
	cc, err := grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Terraformer: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

func callWireguardian(config *pb.Config) (*pb.Config, error) {
	cc, err := grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Wireguardian: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	res, err := wireguardian.BuildVPN(c, &pb.BuildVPNRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

func callKubeEleven(config *pb.Config) (*pb.Config, error) {
	cc, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to KubeEleven: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{Config: config})
	if err != nil {
		return nil, err
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

	for _, cluster := range tmpConfig.GetDesiredState().GetClusters() {
		tmp := make(map[string]*nodesToDelete)
		for _, nodePool := range cluster.GetNodePools() {
			var nodesProvider nodesToDelete
			key := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.Name}

			if _, ok := tableCurrent[key]; ok {
				tmpNodePool := getNodePoolByName(nodePool.Name, utils.GetClusterByName(cluster.Name, tmpConfig.GetDesiredState().GetClusters()).GetNodePools())
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
				log.Println(currentCluster)
				cluster.NodePools = append(cluster.NodePools, getNodePoolByName(key.nodePoolName, currentCluster.GetNodePools()))
				deleting = true
			}
		}
	}

	if adding && deleting {
		return tmpConfig, deleting, delCounts
	} else if deleting {
		return nil, deleting, delCounts
	} else {
		return nil, deleting, nil
	}
}

// getNodePoolByName will return first Nodepool that will have same name as specified in parameters
// If no name is found, return nil
func getNodePoolByName(nodePoolName string, nodePools []*pb.NodePool) *pb.NodePool {
	if nodePoolName == "" {
		return nil
	}
	for i := 0; i < len(nodePools); i++ {
		if nodePools[i].Name == nodePoolName {
			return nodePools[i]
		}
	}
	return nil
}

// processConfig is function used to carry out task specific to Builder concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient, tmp bool) (err error) {
	log.Println("I got config: ", config.GetName())

	config, err = callTerraformer(config)
	if err != nil {
		return
	}

	config, err = callWireguardian(config)
	if err != nil {
		return
	}

	config, err = callKubeEleven(config)
	if err != nil {
		return
	}

	if !tmp {
		config.CurrentState = config.DesiredState // Update currentState
		err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if err != nil {
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
				log.Println("Processing a tmpConfig...")
				err := processConfig(tmpConfig, c, true)
				if err != nil {
					return err
				}
			}
			if deleting {
				log.Println("Deleting nodes...")
				config, err = deleteNodes(config, toDelete)
				if err != nil {
					return err
				}
			}

			log.Println("Processing a config...")
			go func() {
				err := processConfig(config, c, false)
				if err != nil {
					log.Println(err)
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
	_, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Kube-eleven: %v", err)
	}
	_, err = grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Terraformer: %v", err)
	}
	_, err = grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Wireguardian: %v", err)
	}
	return nil
}

func deleteNodes(config *pb.Config, toDelete map[string]*countsToDelete) (*pb.Config, error) {
	for _, cluster := range config.CurrentState.Clusters {
		var nodesToDelete []string
		del := toDelete[cluster.Name]
		for i := len(cluster.NodeInfos) - 1; i >= 0; i-- {
			val, ok := del.nodes[cluster.NodeInfos[i].Provider]
			if ok {
				if val.masterCount > 0 && cluster.NodeInfos[i].IsControl > 0 {
					val.masterCount--
					nodesToDelete = append(nodesToDelete, cluster.NodeInfos[i].GetNodeName())
					continue
				}
				if val.workerCount > 0 && cluster.NodeInfos[i].IsControl == 0 {
					val.workerCount--
					nodesToDelete = append(nodesToDelete, cluster.NodeInfos[i].NodeName)
					continue
				}
			}
		}
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
	var pos int
	for pos = 0; pos < len(slice); pos++ {
		if slice[pos].GetNodeName() == value {
			break
		}
	}
	return append(slice[:pos], slice[pos+1:]...)
}

// deleteNodesByName checks if there is any difference in nodes between a desired state cluster and a running cluster
func deleteNodesByName(cluster *pb.Cluster, nodesToDelete []string) error {
	//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
	for _, node := range nodesToDelete {
		log.Println("kubectl drain " + node + " --ignore-daemonsets --delete-local-data")
		cmd := "kubectl drain " + node + " --ignore-daemonsets --delete-local-data --kubeconfig <(echo '" + cluster.GetKubeconfig() + "')"
		res, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			log.Println("Error while draining node "+node+":", err)
			log.Println(res)
			return err
		}
	}

	//kubectl delete node <node-name>
	for _, node := range nodesToDelete {
		log.Println("kubectl delete node " + node)
		cmd := "kubectl delete node " + node + " --kubeconfig <(echo '" + cluster.GetKubeconfig() + "')"
		_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			log.Println("Error while deleting node "+node+":", err)
			return err
		}
	}
	return nil
}

func main() {
	// If go code crash, we will get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Create connection to Context-box
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Content-box: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker("50051", healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(5*time.Second, ctx, configProcessor(c), worker.ErrorLogger)

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			defer signal.Stop(ch)
			<-ch
			return errors.New("interrupt signal")
		})
	}
	{
		g.Go(func() error {
			w.Run()
			return nil
		})
	}

	log.Println("Stopping Builder: ", g.Wait())
}
