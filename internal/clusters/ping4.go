package clusters

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"golang.org/x/crypto/ssh"
)

const (
	// How long to wait before considering the ping packet to be lost.
	PingTimeout = 2 * time.Second

	// PingRetryCount is the number of times the ping will be retried
	// to determine if the node is healthy of not.
	PingRetryCount = 4
)

var (
	// How many goroutines will be used to ping nodes of a cluster.
	pingConcurrentWorkers = envs.GetOrDefaultInt("PING_CONCURRENT_WORKERS", 10)

	// ErrUnreachable is returned when a node cannot be reached via an SSH ping.
	ErrUnreachable = errors.New("failed to reach node")
)

type NodeEndpoint struct {
	Ip       string
	Port     string
	Username string
	SSHKey   string
}

func (n NodeEndpoint) Incomplete() bool {
	return n.Ip == "" ||
		n.Port == "" ||
		n.Username == "" ||
		n.SSHKey == ""
}

func sshPing(timeout time.Duration, ep NodeEndpoint) error {
	signer, err := ssh.ParsePrivateKey([]byte(ep.SSHKey))
	if err != nil {
		return fmt.Errorf("%s invalid ssh key: %w", ep.Ip, err)
	}

	cfg := ssh.ClientConfig{
		User: ep.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	endpoint := net.JoinHostPort(ep.Ip, ep.Port)

	//nolint
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set connection timeouts: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, endpoint, &cfg)
	if err != nil {
		return fmt.Errorf("failed to establish ssh client connection: %w", err)
	}
	return ssh.NewClient(c, chans, reqs).Close()
}

// SSHPing performs an SSH ping against an [NodeEndpoint] with the requested amount of retries.
func SSHPing(logger zerolog.Logger, retries int, dst NodeEndpoint) error {
	if dst.Incomplete() {
		logger.
			Warn().
			Msgf("Received incomplete node endpoint address to ssh ping %q, considering as unreachable", dst.Ip)

		return fmt.Errorf("unhealthy connection: %w, IP address %q", ErrUnreachable, dst.Ip)
	}

	var err error
	for i := range retries {
		logger.Debug().Msgf("SSH ping %s:%s", dst.Ip, dst.Port)
		if err = sshPing(PingTimeout, dst); err == nil {
			break
		}
		if i == retries-1 {
			break
		}
		wait := 1 * time.Second
		logger.Warn().Msgf("failed to ssh ping node %q: %v, retrying again in %s", dst.Ip, err, wait)
		time.Sleep(wait)
	}
	if err != nil {
		return fmt.Errorf("unhealthy connection: %w: %w", ErrUnreachable, err)
	}
	return nil
}

// PingLoadBalancerNodes pings all of the nodes of the LoadBalancer cluster and
// returns those that are unreachable.
//
// The resulting value will be of map[LoadBalancer.Id]map[NodePool.Name][]NodeIPs.
func PingLoadBalancerNodes(logger zerolog.Logger, state *spec.Clusters) (map[string]map[string][]string, error) {
	type nodemap = map[NodeEndpoint]string  // map[NodeEndpoint]NodePool.Name
	nodepoolMap := make(map[string]nodemap) // map[LoadBalancer.Id]nodemap

	for _, lb := range state.GetLoadBalancers().GetClusters() {
		nodepoolMap[lb.ClusterInfo.Id()] = make(nodemap)

		for _, np := range lb.ClusterInfo.NodePools {
			for _, n := range np.Nodes {
				ep := NodeEndpoint{
					Ip:       n.Public,
					Port:     fmt.Sprint(nodepools.NodeSSHPort(np, n)),
					Username: nodepools.NodeSSHUsername(n),
					SSHKey:   "",
				}

				switch t := np.Type.(type) {
				case *spec.NodePool_DynamicNodePool:
					ep.SSHKey = t.DynamicNodePool.PrivateKey
				case *spec.NodePool_StaticNodePool:
					ep.SSHKey = t.StaticNodePool.NodeKeys[n.Public]
				}

				nodepoolMap[lb.ClusterInfo.Id()][ep] = np.Name
			}
		}
	}

	var eps []NodeEndpoint
	for _, e := range nodepoolMap {
		eps = slices.AppendSeq(eps, maps.Keys(e))
	}

	unreachable, err := pingAll(logger, pingConcurrentWorkers, eps, SSHPing)

	out := make(map[string]map[string][]string) // map[LoadBalancer.Id]map[NodePool.Name][]NodeIP
	for _, ep := range unreachable {
		for id, nodepools := range nodepoolMap {
			if nodepoolName, ok := nodepools[ep]; ok {
				if out[id] == nil {
					out[id] = make(map[string][]string)
				}
				out[id][nodepoolName] = append(out[id][nodepoolName], ep.Ip)
			}
		}
	}
	return out, err
}

func pingAll(
	logger zerolog.Logger,
	goroutineCount int,
	eps []NodeEndpoint,
	f func(logger zerolog.Logger, count int, ep NodeEndpoint) error,
) ([]NodeEndpoint, error) {
	if goroutineCount < 1 {
		return nil, nil
	}

	type errResult struct {
		ep  NodeEndpoint
		err error
	}

	var (
		wg      = new(sync.WaitGroup)
		errChan = make(chan errResult)
		tasks   = make(chan NodeEndpoint)
	)

	for range goroutineCount {
		wg.Go(func() {
			for ep := range tasks {
				if err := f(logger, PingRetryCount, ep); err != nil {
					errChan <- errResult{
						ep:  ep,
						err: err,
					}
				}
			}
		})
	}

	go func() {
		for _, ep := range eps {
			tasks <- ep
		}
		close(tasks)
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var unreachable []NodeEndpoint
	var errAll error
	for result := range errChan {
		unreachable = append(unreachable, result.ep)
		errAll = errors.Join(errAll, fmt.Errorf("node %s: %w", result.ep.Ip, result.err))
	}
	return unreachable, errAll
}
