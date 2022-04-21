package testingframework

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"

	"io/ioutil"
	"testing"
)

const (
	testDir          = "tests"
	maxTimeout       = 3600 // max allowed time for one manifest to finish in [seconds]
	maxLonghornCheck = 240  // max allowed time for pods of longhorn-system to be ready [seconds]
	sleepSec         = 30   // seconds for one cycle of config check
	sleepSecPods     = 10   // seconds for one cycle of longhorn checks (the node and pod checks)
)

// ClientConnection will return new client connection to Context-box
func ClientConnection() pb.ContextBoxServiceClient {
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
}

// TestPlatform will start all the test cases specified in tests directory
func TestPlatform(t *testing.T) {
	c := ClientConnection()
	log.Info().Msg("----Starting the tests----")

	// loop through the directory and list files inside
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test sets: %v", err)
	}

	// save all the test set paths
	var pathsToSets []string
	for _, f := range files {
		if f.IsDir() {
			log.Info().Msgf("Found test set: %s", f.Name())
			setTestDir := filepath.Join(testDir, f.Name())
			pathsToSets = append(pathsToSets, setTestDir)
		}
	}

	for _, path := range pathsToSets {
		err := applyTestSet(path, c)
		if err != nil {
			t.Logf("Error while processing %s : %v", path, err)
			t.Fail()
		}
	}
}

// applyTestSet function will apply test set sequantially to a platform
func applyTestSet(pathToSet string, c pb.ContextBoxServiceClient) error {
	done := make(chan string)
	var id string

	log.Info().Msgf("Working on the test set: %s", pathToSet)

	files, err := ioutil.ReadDir(pathToSet)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test configs: %v", err)
	}

	for _, file := range files {
		setFile := filepath.Join(pathToSet, file.Name())
		manifest, errR := ioutil.ReadFile(setFile)
		if errR != nil {
			log.Fatal().Err(errR)
		}

		id, err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     file.Name(),
				Id:       id,
				Manifest: string(manifest),
			},
		})

		if err != nil {
			log.Fatal().Msgf("Error while saving a config: %v", err)
			return err
		}
		go configChecker(done, c, id, file.Name(), pathToSet)
		// wait until test config has been processed
		if res := <-done; res != "ok" {
			log.Error().Msg(res)
			return fmt.Errorf(res)
		}
	}
	// clean up
	log.Info().Msgf("Deleting the clusters from test set: %s", pathToSet)
	err = cbox.DeleteConfig(c, id)
	if err != nil {
		return err
	}

	return nil
}

// configChecker function will check if the config has been applied every 30s
func configChecker(done chan string, c pb.ContextBoxServiceClient, configID, configName, testSetName string) {
	counter := 1
	for {
		elapsedSec := counter * sleepSec
		// if CSchecksum == DSchecksum, the config has been processed
		config, err := c.GetConfigById(context.Background(), &pb.GetConfigByIdRequest{
			Id: configID,
		})
		if err != nil {
			log.Fatal().Msg(fmt.Sprintf("Got error while waiting for config to finish: %v", err))
		}
		if config != nil {
			cfg := config.Config
			if len(cfg.ErrorMessage) > 0 {
				emsg := cfg.ErrorMessage
				log.Error().Msg(emsg)
				done <- emsg
				return
			}
			cs := cfg.CsChecksum
			ds := cfg.DsChecksum
			if checksumsEqual(cs, ds) {
				//start longhorn testing
				clusters := cfg.CurrentState.Clusters
				for _, cluster := range clusters {
					// check number of nodes in nodes.longhorn.io
					err := checkLonghornNodes(cluster)
					if err != nil {
						emsg := fmt.Sprintf("error while checking the nodes.longhorn.io : %v", err)
						log.Fatal().Msg(emsg)
						done <- emsg
					}
					// check if all pods from longhorn-system are ready
					err = checkLonghornPods(cluster.Kubeconfig)
					if err != nil {
						emsg := fmt.Sprintf("error while checking if all pods from longhorn-system are ready : %v", err)
						log.Fatal().Msg(emsg)
						done <- emsg
					}
				}
				break
			}
		}
		if elapsedSec == maxTimeout {
			emsg := fmt.Sprintf("Test took too long... Aborting on timeout %d seconds", maxTimeout)
			log.Fatal().Msg(emsg)
			done <- emsg
			return
		}
		time.Sleep(time.Duration(sleepSec) * time.Second)
		counter++
		log.Info().Msgf("Waiting for %s to from %s finish... [ %ds elapsed ]", configName, testSetName, elapsedSec)
	}
	// send signal that config has been processed, unblock the applyTestSet
	done <- "ok"
}

// checksumsEq will check if two checksums are equal
func checksumsEqual(checksum1 []byte, checksum2 []byte) bool {
	if len(checksum1) > 0 && len(checksum2) > 0 && bytes.Equal(checksum1, checksum2) {
		return true
	} else {
		return false
	}
}

// checkLonghornNodes will check if the count of nodes.longhorn.io is same as number of schedulable nodes
func checkLonghornNodes(cluster *pb.K8Scluster) error {
	command := fmt.Sprintf("kubectl get nodes.longhorn.io -A -o json --kubeconfig <(echo '%s') | jq -r '.items | length '", cluster.Kubeconfig)
	allNodesFound := false
	readyCheck := 0
	workerCount := 0
	count := "" //in order to save last value for error message, the var is defined here
	//count the worker nodes
	for _, nodepool := range cluster.ClusterInfo.NodePools {
		if !nodepool.IsControl {
			workerCount += int(nodepool.Count)
		}
	}
	// give them time of maxLonghornCheck seconds to be scheduled
	for readyCheck < maxLonghornCheck {
		cmd := exec.Command("/bin/bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			emsg := fmt.Sprintf("error while getting the nodes.longhorn.io count %v", err)
			return fmt.Errorf(emsg)
		}
		//trim the whitespaces
		count = strings.Trim(string(out), "\t\n ")
		// the number of worker nodes should be equal to number of scheduled nodes in longhorn
		// NOTE: by default, master nodes will not be used to schedule pods, however, if this changes the condition will be broken
		if count != fmt.Sprint(workerCount) {
			readyCheck += sleepSecPods
		} else {
			allNodesFound = true
			break
		}
		time.Sleep(time.Duration(sleepSecPods) * time.Second)
		log.Info().Msgf("Waiting for nodes.longhorn.io to be initialized... [ %ds elapsed ]", readyCheck)
	}
	if !allNodesFound {
		return fmt.Errorf(fmt.Sprintf("the count of schedulable nodes (%d) is not equal to nodes.longhorn.io (%s)", workerCount, count))
	}
	return nil
}

// checkLonghornPods will check if the pods in longhorn-system namespace are in ready state
func checkLonghornPods(config string) error {
	command := fmt.Sprintf("kubectl get pods -n longhorn-system -o json --kubeconfig <(echo '%s') | jq -r '.items[] | .status.containerStatuses[].ready'", config)
	readyCheck := 0
	allPodsReady := false
	// give them time of maxLonghornCheck seconds to be scheduled
	for readyCheck < maxLonghornCheck {
		cmd := exec.Command("/bin/bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("error while getting the status of the pods in longhorn-system %v", err))
		}
		// if some are not ready, wait sleepSecPods seconds
		if strings.Contains(string(out), "false") {
			readyCheck += sleepSecPods
		} else {
			allPodsReady = true
			break
		}
		time.Sleep(time.Duration(sleepSecPods) * time.Second)
		log.Info().Msgf("Waiting for pods from longhorn-system namespace to be in ready state... [ %ds elapsed ]", readyCheck)
	}
	if !allPodsReady {
		return fmt.Errorf("pods in longhorn-system took too long to initialize")
	}
	return nil
}
