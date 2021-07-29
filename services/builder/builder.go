package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"

	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"github.com/Berops/platform/urls"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func callTerraformer(config *pb.Config) *pb.Config {
	// Create connection to Terraformer
	cc, err := grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Terraformer: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}

	return res.GetConfig()
}

func callWireguardian(config *pb.Config) *pb.Config {
	cc, err := grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Wireguardian: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	res, err := wireguardian.BuildVPN(c, &pb.BuildVPNRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}
	return res.GetConfig()
}

func callKubeEleven(config *pb.Config) *pb.Config {
	cc, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to KubeEleven: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}
	return res.GetConfig()
}

func diff(config *pb.Config) *pb.Config {
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

	var tableCurrent = make(map[tableKey]nodeCount)
	for _, cluster := range tmpConfig.GetCurrentState().GetClusters() {
		for _, nodePool := range cluster.GetNodePools() {
			tmp := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.Name}
			tableCurrent[tmp] = nodeCount{nodePool.Master.Count, nodePool.Worker.Count}
		}
	}

	for _, cluster := range tmpConfig.GetDesiredState().GetClusters() {
		for _, nodePool := range cluster.GetNodePools() {

			key := tableKey{nodePoolName: nodePool.Name, clusterName: cluster.Name}

			if _, ok := tableCurrent[key]; ok {
				tmpNodePool := getNodePoolByName(nodePool.Name, getClusterByName(cluster.Name, tmpConfig.GetDesiredState().GetClusters()).GetNodePools())
				if nodePool.Master.Count > tableCurrent[key].masterCount {
					tmpNodePool.Master.Count = nodePool.Master.Count
					adding = true
				} else if nodePool.Master.Count < tableCurrent[key].masterCount {
					tmpNodePool.Master.Count = tableCurrent[key].masterCount
					deleting = true
				}
				if nodePool.Worker.Count > tableCurrent[key].workerCount {
					tmpNodePool.Worker.Count = nodePool.Worker.Count
					adding = true
				} else if nodePool.Worker.Count < tableCurrent[key].workerCount {
					tmpNodePool.Worker.Count = tableCurrent[key].workerCount
					deleting = true
				}
				delete(tableCurrent, key)
			}
		}
	}

	if len(tableCurrent) > 0 {
		for key := range tableCurrent {
			cluster := getClusterByName(key.clusterName, tmpConfig.DesiredState.Clusters)
			if cluster != nil {
				currentCluster := getClusterByName(key.clusterName, tmpConfig.CurrentState.Clusters)
				log.Println(currentCluster)
				cluster.NodePools = append(cluster.NodePools, getNodePoolByName(key.nodePoolName, currentCluster.GetNodePools()))
				deleting = true
			}
		}
	}

	if adding && deleting {
		return tmpConfig
	} else {
		return nil
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

// getClusterByName will return Cluster that will have same name as specified in parameters
// If no name is found, return nil
func getClusterByName(clusterName string, clusters []*pb.Cluster) *pb.Cluster {
	if clusterName == "" {
		return nil
	}
	for i := 0; i < len(clusters); i++ {
		if clusters[i].Name == clusterName {
			return clusters[i]
		}
	}
	return nil
}

// flow will initiate kubernetes cluster creation cycle
func flow(config *pb.Config) *pb.Config {
	config = callTerraformer(config)
	config = callWireguardian(config)
	config = callKubeEleven(config)
	return config
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
	go func() {
		for {
			res, err := cbox.GetConfigBuilder(c) // Get a new config
			if err != nil {
				log.Fatalln("Error while getting config from the Builder", err)
			}
			if res.GetConfig() != nil {
				var tmpConfig *pb.Config
				config := res.GetConfig()
				log.Println("I got config: ", config.GetCurrentState())
				if len(config.CurrentState.GetClusters()) > 0 {
					tmpConfig = diff(config)
				}
				if tmpConfig != nil {
					flow(tmpConfig)
				}
				config = flow(config)
				log.Println("SAVING DESIRED STATE AS CURRENT STATE")
				config.CurrentState = config.DesiredState // Update currentState
				err = cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
				if err != nil {
					log.Fatalln("Error while saving the config", err)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	fmt.Println("Stopping Builder")
}
