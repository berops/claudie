package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ErrEchoTimeout is returned when the reply is not received within the requested timeout.
var ErrEchoTimeout = errors.New("icmp request timeout")

func ping4(conn *icmp.PacketConn, id, seq int, dst *net.UDPAddr, timeout time.Duration) error {
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
				log.Err(err).Msgf("failed to set read deadline for icmp connection")
				continue
			}

			rcv := make([]byte, 1024)
			r, peer, err := conn.ReadFrom(rcv)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					return ErrEchoTimeout
				}
				log.Err(err).Msgf("failed to read icmp seq %v reply", seq)
				continue
			}

			if peer.String() != dst.String() {
				continue
			}

			reply, err := icmp.ParseMessage(1, rcv[:r])
			if err != nil {
				log.Err(err).Msgf("failed to parse icmp seq %v reply", seq)
				continue
			}

			switch reply.Type {
			case ipv4.ICMPTypeEchoReply:
				body, ok := reply.Body.(*icmp.Echo)
				if !ok {
					log.Debug().Msg("received icmp echo reply does not have expected echo body, skipping")
					continue
				}
				if body.ID != id {
					log.Debug().Msgf("recieved icmp echo reply id: %v does not match echo request id: %v, skipping\n", body.ID, id)
					continue
				}
				if body.Seq != seq {
					log.Debug().Msgf("received icmp echo reply seq: %v does not match echo request seq: %v, skipping\n", body.Seq, seq)
					continue
				}
				return nil
			default:
				fmt.Printf("received non icmp echo reply packet, skipping\n")
			}
		}
	}
}

func ping(count int, dst string) error {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("failed to listen for icmp packets: %w", err)
	}
	defer conn.Close()

	dstAddr := &net.UDPAddr{IP: net.ParseIP(dst)}
	timeout := 4 * time.Second
	id := os.Getpid() & 0xffff // 16bit id
	lost := 0

	log.Info().Msgf("verifying if peer %s is reachable", dst)
	for i := range count {
		time.Sleep(1 * time.Second)
		seq := i + 1
		send := time.Now()
		if err := ping4(conn, id, seq, dstAddr, timeout); err != nil {
			if errors.Is(err, ErrEchoTimeout) {
				log.Warn().Msgf("[%v] icmp seq %v, lost\n", time.Since(send).String(), seq)
				lost++
				continue
			}
			log.Err(err).Msgf("failed to ping peer %s", dst)
			continue
		}
		log.Info().Msgf("[%v] icmp seq %v, ok\n", time.Since(send).String(), seq)
	}

	if lost > count/2 {
		return fmt.Errorf("unhealthy connection, [%v/%v] messages lost", lost, count)
	}
	return nil
}

func AllNodesReachable(state *spec.Clusters) ([]string, error) {
	if state == nil {
		return nil, nil
	}

	// collect all ips first and then distribute them among multiple goroutines
	// have a fixed number of goroutines though like maybe 3 top...
	// TODO: rename peer to node.
	// TODO figure out how to only error out a single cluster
	// such that the error will also be read via the kube controller.

	// todo: parralelize.
	for _, np := range state.K8S.ClusterInfo.NodePools {
		for _, n := range np.Nodes {
			if err := ping(6, n.Public); err != nil {
				return []string{n.Public}, err
			}
		}
	}

	return nil, nil
}
