package clusters

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	// How long to wait before considering the ping packet to be lost.
	pingTimeout = 3 * time.Second
	// PingRetryCount is the number of times the ping will be retried
	// to determine if the node is healthy of not.
	PingRetryCount = 5
)

// How many goroutines will be used to ping nodes of a cluster.
var pingConcurrentWorkers = envs.GetOrDefaultInt("PING_CONCURRENT_WORKERS", 20)

// ErrEchoTimeout is returned when the reply is not received within the requested timeout.
var ErrEchoTimeout = errors.New("icmp request timeout")

func ping4(logger zerolog.Logger, conn *icmp.PacketConn, id, seq int, dst *net.UDPAddr, timeout time.Duration) error {
	m := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("ping"),
		},
	}

	b, err := m.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to marshal icmp seq %v: %w", seq, err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set write deadline for icmp seq: %v: %w", seq, err)
	}

	w, err := conn.WriteTo(b, dst)
	if err != nil {
		return fmt.Errorf("failed to write icmp seq %v: %w", seq, err)
	}

	if w != len(b) {
		return fmt.Errorf("not all bytes for icmp seq %v were written", seq)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ErrEchoTimeout
		default:
			if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
				logger.Err(err).Msgf("Failed to set read deadline for icmp connection")
				continue
			}

			rcv := make([]byte, 1024)
			r, peer, err := conn.ReadFrom(rcv)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					return ErrEchoTimeout
				}
				logger.Err(err).Msgf("Failed to read icmp seq %v reply", seq)
				continue
			}

			if peer.String() != dst.String() {
				continue
			}

			reply, err := icmp.ParseMessage(1, rcv[:r]) // 1 is icmp ip4 protocol.
			if err != nil {
				logger.Err(err).Msgf("failed to parse icmp seq %v reply", seq)
				continue
			}

			switch reply.Type {
			case ipv4.ICMPTypeEchoReply:
				body, ok := reply.Body.(*icmp.Echo)
				if !ok {
					logger.Warn().Msg("Received icmp echo reply does not have expected echo body, skipping")
					continue
				}

				// Reply ID seems to be different when executed inside a container
				// can't really match the request via the reply ID, thus we only
				// have the seq number to match the reply to the request.
				// As long as there are no two concurrent ping attempts to the
				// same peer this should be fine.
				// if body.ID != id {
				// 	logger.Debug().Msgf("Received icmp echo reply id: %v does not match echo request id: %v, skipping", body.ID, id)
				// 	continue
				// }
				if body.Seq != seq {
					logger.Warn().Msgf("Received icmp echo reply seq: %v does not match echo request seq: %v, skipping", body.Seq, seq)
					continue
				}
				return nil
			default:
				logger.Debug().Msgf("Received non icmp echo reply packet, skipping")
			}
		}
	}
}

// Ping pings a single IPv4 address with the requested amount of retries.
// An error is returned when more then count/2 packets are lost.
func Ping(logger zerolog.Logger, count int, dst string) error {
	dstAddr := &net.UDPAddr{IP: net.ParseIP(dst)}
	if dstAddr.IP == nil {
		logger.Warn().Msgf("Received invalid IP address to ping %q, skipping unreachability check", dst)
		return nil
	}

	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("failed to listen for icmp packets: %w", err)
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff // 16bit id
	lost := 0

	logger.Debug().Msgf("verifying if node %s is reachable", dst)
	for i := range count {
		time.Sleep(1 * time.Second)
		seq := i + 1
		send := time.Now()
		if err := ping4(logger, conn, id, seq, dstAddr, pingTimeout); err != nil {
			lost++
			if errors.Is(err, ErrEchoTimeout) {
				logger.Warn().Msgf("[%v] node %s icmp seq %v, lost", time.Since(send).String(), dst, seq)
				continue
			}
			logger.Err(err).Msg("failed to ping node")
			continue
		}
		logger.Debug().Msgf("[%v] node %s icmp seq %v, ok", time.Since(send).String(), dst, seq)
	}
	if lost > max(count/2, (count+1)/2) {
		return fmt.Errorf("unhealthy connection: %w, [%v/%v] messages lost", ErrEchoTimeout, lost, count)
	}
	return nil
}

// PingNodes pings nodes of the cluster, including loadbalancer nodes, using
// the public IPv4 Address of the nodes.
func PingNodes(logger zerolog.Logger, state *spec.Clusters) (map[string][]string, map[string]map[string][]string, error) {
	type nodemap = map[string]string

	k8sNodes := make(nodemap)
	for _, np := range state.GetK8S().GetClusterInfo().GetNodePools() {
		for _, n := range np.Nodes {
			k8sNodes[n.Public] = np.Name
		}
	}

	lbsNodes := make(map[string]nodemap)
	for _, lb := range state.GetLoadBalancers().GetClusters() {
		lbsNodes[lb.ClusterInfo.Id()] = make(nodemap)
		for _, np := range lb.GetClusterInfo().GetNodePools() {
			for _, n := range np.Nodes {
				lbsNodes[lb.ClusterInfo.Id()][n.Public] = np.Name
			}
		}
	}

	ips := slices.Collect(maps.Keys(k8sNodes))
	for _, lbs := range lbsNodes {
		ips = slices.AppendSeq(ips, maps.Keys(lbs))
	}

	k8sip := make(map[string][]string)
	lbsip := make(map[string]map[string][]string)

	unreachable, err := pingAll(logger, pingConcurrentWorkers, ips, Ping)
	for _, ip := range unreachable {
		if np, ok := k8sNodes[ip]; ok {
			k8sip[np] = append(k8sip[np], ip)
		} else {
			for lb, nodes := range lbsNodes {
				if np, ok := nodes[ip]; ok {
					if lbsip[lb] == nil {
						lbsip[lb] = make(map[string][]string)
					}
					lbsip[lb][np] = append(lbsip[lb][np], ip)
				}
			}
		}
	}

	return k8sip, lbsip, err
}

func pingAll(
	logger zerolog.Logger,
	goroutineCount int,
	ips []string,
	f func(logger zerolog.Logger, count int, dst string) error,
) ([]string, error) {
	if goroutineCount < 1 {
		return nil, nil
	}

	type data struct {
		ip  string
		err error
	}

	var (
		wg      = new(sync.WaitGroup)
		errChan = make(chan data)
		tasks   = make(chan string)
	)

	for range goroutineCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range tasks {
				if err := f(logger, PingRetryCount, ip); err != nil {
					errChan <- data{
						ip:  ip,
						err: err,
					}
				}
			}
		}()
	}

	go func() {
		for _, ip := range ips {
			tasks <- ip
		}
		close(tasks)
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var unreachable []string
	var errAll error
	for err := range errChan {
		unreachable = append(unreachable, err.ip)
		errAll = errors.Join(errAll, fmt.Errorf("node %s: %w", err.ip, err.err))
	}
	return unreachable, errAll
}
