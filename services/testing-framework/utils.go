package testingframework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"

	"gopkg.in/yaml.v3"
)

const (
	maxTimeout     = 24_500  // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 8       // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

var errInterrupt = errors.New("interrupt")

type testset struct{ Config, Set, Manifest string }

func waitForDoneOrError(ctx context.Context, manager managerclient.CrudAPI, set testset) (*spec.Config, error) {
	elapsed := 0
	ticker := time.NewTicker(sleepSec * time.Second)
	defer ticker.Stop()

	// How many reconciliation iterations are needed for a definitive answer
	// of whether the input manifest is in error or done.
	const iterationsNeeded = 10

	var done int
	var failed int

	for {
		select {
		case <-ctx.Done():
			return nil, errInterrupt
			// This is triggered every 10 seconds and the reconciliation loop is triggered every 20 seconds
			// With these timeouts the 9 iterations needed should be enough for deciding if its done or error.
		case <-ticker.C:
			elapsed += sleepSec
			log.Info().Msgf("Waiting for %s from %s to finish... [ %ds elapsed ]", set.Manifest, set.Set, elapsed)
			if elapsed >= maxTimeout {
				return nil, fmt.Errorf("test took too long... Aborting after %d seconds", maxTimeout)
			}

			res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: set.Config})
			if err != nil {
				return nil, fmt.Errorf("error while waiting for config to finish: %w", err)
			}

			switch res.Config.Manifest.State {
			case spec.Manifest_Scheduled:
				done = 0

				// On scheduled only reset the done counter as the failed
				// counter needs to stay the same if it is crashlooping in a
				// reconciliation loop.
			case spec.Manifest_Pending:
				clustersErrored := 0
				for _, s := range res.Config.Clusters {
					// When the input manifest is in Pending
					// it is sufficient to check for [spec.Workflow_ERROR]
					// to determined if it failed as the other possible
					// state is [spec.Workflow_DONE]
					if s.State.Status == spec.Workflow_ERROR {
						clustersErrored += 1
					}
				}

				if clustersErrored == 0 {
					failed = 0
					done += 1

					if done == iterationsNeeded {
						if err := validateState(ctx, res.Config.Clusters); err != nil {
							return nil, fmt.Errorf("failed to validate current state: %w", err)
						}
						return res.Config, nil
					}
				} else {
					failed += 1
					done = 0

					if failed == iterationsNeeded {
						err := fmt.Errorf(
							"%q had a cluster with a pending InFlight state for longer than the specified timeout", res.Config.Name,
						)

						if validateErr := validateState(ctx, res.Config.Clusters); validateErr != nil {
							err = errors.Join(err, validateErr)
						}

						for cluster, state := range res.Config.Clusters {
							if state.State.Status == spec.Workflow_ERROR {
								err = errors.Join(
									err,
									fmt.Errorf("----\nerror in cluster %s\n----\nStage: %v \n State: %s\n Description: %s", cluster, state.InFlight.CurrentStage, state.State.Status, state.State.Description),
								)
							}
						}
						return nil, err
					}
				}
			case spec.Manifest_Done:
				failed = 0
				done += 1

				if done == iterationsNeeded {
					if err := validateState(ctx, res.Config.Clusters); err != nil {
						return nil, fmt.Errorf("failed to validate current state: %w", err)
					}
					return res.Config, nil
				}
			case spec.Manifest_Error:
				failed += 1
				done = 0

				if failed == iterationsNeeded {
					err := errors.New("input manifest failed")

					if validateErr := validateState(ctx, res.Config.Clusters); validateErr != nil {
						err = errors.Join(err, validateErr)
					}

					for cluster, state := range res.Config.Clusters {
						if state.State.Status == spec.Workflow_ERROR {
							err = errors.Join(
								err,
								fmt.Errorf("----\nerror in cluster %s\n----\nStage: %v \n State: %s\n Description: %s", cluster, state.InFlight.CurrentStage, state.State.Status, state.State.Description),
							)
						}
					}
					return nil, err
				}
			}
		}
	}
}

func getAutoscaledClusters(c *spec.Config) []*spec.K8Scluster {
	clusters := make([]*spec.K8Scluster, 0, len(c.Clusters))

	for _, s := range c.Clusters {
		if s.Current != nil && len(nodepools.Autoscaled(s.Current.K8S.ClusterInfo.NodePools)) > 0 {
			clusters = append(clusters, s.Current.GetK8S())
		}
	}

	return clusters
}

func validateKubeconfigAlternativeNames(c map[string]*spec.ClusterState) error {
	for c, v := range c {
		if v.Current == nil || v.Current.K8S == nil || v.Current.K8S.Kubeconfig == "" {
			continue
		}
		// if the clusters has no APIServer Loadbalancer we can test all
		// control plane nodes to validate if they all can be used with the
		// generated KubeConfig.
		if clusters.FindAssignedLbApiEndpoint(v.GetCurrent().GetLoadBalancers().GetClusters()) != nil {
			continue
		}

		var kubeconfigs []string

		kubeconfig := map[string]interface{}{}
		if err := yaml.Unmarshal([]byte(v.Current.K8S.Kubeconfig), &kubeconfig); err != nil {
			return fmt.Errorf("cluster %q: %w", c, err)
		}

		cluster := kubeconfig["clusters"].([]interface{})[0]
		clusterMap := cluster.(map[string]interface{})["cluster"].(map[string]interface{})
		for _, n := range v.Current.K8S.ClusterInfo.NodePools {
			if !n.IsControl {
				continue
			}

			for _, n := range n.Nodes {
				clusterMap["server"] = fmt.Sprintf("https://%s:6443", n.Public)
				newConfig, err := yaml.Marshal(kubeconfig)
				if err != nil {
					return fmt.Errorf("cluster %q: %w", c, err)
				}

				kubeconfigs = append(kubeconfigs, string(newConfig))
			}
		}

		var output []byte
		for _, kubeconfig := range kubeconfigs {
			k := kubectl.Kubectl{
				Kubeconfig:        kubeconfig,
				MaxKubectlRetries: 5,
			}
			nodes, err := k.KubectlGetNodeNames()
			if err != nil {
				return fmt.Errorf("cluster %q: %w", c, err)
			}

			// initialize only once, every output should then
			// be the same.
			if output == nil {
				output = nodes
			}

			if !bytes.Equal(nodes, output) {
				return fmt.Errorf("cluster %q does not have kubeconfig signed for all control plane nodes", c)
			}
		}
	}

	return nil
}

func validateState(ctx context.Context, clusters map[string]*spec.ClusterState) error {
	if err := validateKubeconfigAlternativeNames(clusters); err != nil {
		return err
	}

	// For each node verify that the wireguard peers are matching the current state.
	for _, v := range clusters {
		if v.Current == nil || v.Current.K8S == nil || v.Current.K8S.Kubeconfig == "" {
			continue
		}

		expectedPeerList := buildExpectedPeerList(v)

		// validate lbs
		for _, lb := range v.Current.LoadBalancers.Clusters {
			if err := validateWireguardSetup(lb.ClusterInfo.NodePools, expectedPeerList); err != nil {
				return err
			}
		}

		// validate k8s nodes.
		if err := validateWireguardSetup(v.Current.K8S.ClusterInfo.NodePools, expectedPeerList); err != nil {
			return err
		}
	}

	return testLonghornDeployment(ctx, clusters)
}

type Peer struct {
	Public  string
	Private string
}

func buildExpectedPeerList(cluster *spec.ClusterState) []Peer {
	var out []Peer

	for _, lb := range cluster.Current.LoadBalancers.Clusters {
		for _, np := range lb.ClusterInfo.NodePools {
			for _, n := range np.Nodes {
				out = append(out, Peer{
					Public:  n.Public,
					Private: n.Private,
				})
			}
		}
	}

	for _, np := range cluster.Current.K8S.ClusterInfo.NodePools {
		for _, n := range np.Nodes {
			out = append(out, Peer{
				Public:  n.Public,
				Private: n.Private,
			})
		}
	}

	return out
}

func validateWireguardSetup(nps []*spec.NodePool, expectedPeerList []Peer) error {
	for _, np := range nps {
		for _, n := range np.Nodes {
			var sshKey string
			username := n.Username
			if username == "" {
				username = "root"
			}

			switch typ := np.Type.(type) {
			case *spec.NodePool_DynamicNodePool:
				sshKey = typ.DynamicNodePool.PrivateKey
			case *spec.NodePool_StaticNodePool:
				sshKey = typ.StaticNodePool.NodeKeys[n.Public]
			default:
				panic(fmt.Sprintf("unexpected spec.isNodePool_Type: %#v", typ))
			}

			signer, err := ssh.ParsePrivateKey([]byte(sshKey))
			if err != nil {
				return fmt.Errorf("node %q has an invalid private key: %w", n.Name, err)
			}

			cfg := ssh.ClientConfig{
				User: username,
				Auth: []ssh.AuthMethod{
					ssh.PublicKeys(signer),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),

				// A single ping can take up to ~330ms across
				// the globe, 2 seconds should be generous for
				// establishing the TCP connection.
				Timeout: 2 * time.Second,
			}

			if err := checkWireguardPeers(n, expectedPeerList, &cfg); err != nil {
				return fmt.Errorf("wireguard peer check failed: %w", err)
			}
		}
	}

	return nil
}

func checkWireguardPeers(thisNode *spec.Node, peerList []Peer, cfg *ssh.ClientConfig) error {
	const sshPort = "22"
	endpoint := net.JoinHostPort(thisNode.Public, sshPort)
	client, err := ssh.Dial("tcp", endpoint, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// According to the documentation the output of wireguard is safe to be
	// parsed, even within scripts
	// https://manpages.debian.org/unstable/wireguard-tools/wg.8.en.html#show
	b, err := session.Output("wg show all dump")
	if err != nil {
		return err
	}

	output := strings.TrimSpace(string(b))

	var fetchedPeers []string
	for line := range strings.SplitSeq(output, "\n") {
		line := strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 9 {
			continue
		}
		fetchedPeers = append(fetchedPeers, line)
	}

	expectedPeerList := slices.Clone(peerList)

	// exclude self from list
	expectedPeerList = slices.DeleteFunc(
		expectedPeerList,
		func(s Peer) bool { return s.Public == thisNode.Public && s.Private == thisNode.Private },
	)

	if len(fetchedPeers) != len(expectedPeerList) {
		return fmt.Errorf(
			"mismatched wireguard peer list %v:%v",
			len(fetchedPeers),
			len(expectedPeerList),
		)
	}

	// go over the expected peers and match a line in the fetched peers.
	for _, p := range expectedPeerList {
		ok := slices.ContainsFunc(fetchedPeers, func(s string) bool {
			return strings.Contains(s, p.Public) && strings.Contains(s, p.Private)
		})
		if !ok {
			return fmt.Errorf("peer %#v is missing from node %q", p, thisNode.Name)
		}
	}

	return nil
}
