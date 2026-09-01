package clusters

import (
	"bytes"
	"crypto/ed25519"
	crand "crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"
)

func FuzzPingAll(f *testing.F) {
	testcases := []string{
		"1,2,3",
		"1,2,3,4,5",
		"1,2,3,4,5,6,7,8,9,10",
		"1,2,3,4,5,6,7,8,9,16,17,18",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}
	log := zerolog.Logger{}

	f.Fuzz(func(t *testing.T, s string) {
		var eps []NodeEndpoint
		for ip := range strings.SplitSeq(s, ",") {
			eps = append(eps, NodeEndpoint{Ip: ip})
		}
		gc := rand.IntN(20)
		u, err := pingAll(log, gc, eps, func(logger zerolog.Logger, count int, ep NodeEndpoint) error { return nil })
		if err != nil {
			t.Errorf("pingAll() goroutines = %v, eps = %v, unreachable = %v, err = %v", gc, eps, u, err)
		}
	})
}

func TestPingAll(t *testing.T) {
	logger := zerolog.Logger{}

	eps := func(ips ...string) []NodeEndpoint {
		out := make([]NodeEndpoint, 0, len(ips))
		for _, ip := range ips {
			out = append(out, NodeEndpoint{Ip: ip})
		}
		return out
	}

	ok := func(logger zerolog.Logger, count int, ep NodeEndpoint) error { return nil }
	fail := func(ips ...string) func(logger zerolog.Logger, count int, ep NodeEndpoint) error {
		return func(logger zerolog.Logger, count int, ep NodeEndpoint) error {
			if slices.Contains(ips, ep.Ip) {
				return errors.New("not reachable")
			}
			return nil
		}
	}

	tests := []struct {
		name           string
		goroutineCount int
		eps            []NodeEndpoint
		f              func(logger zerolog.Logger, count int, ep NodeEndpoint) error
		want           []string
		wantErr        bool
	}{
		{
			goroutineCount: 0,
			eps:            eps(),
			f:              ok,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			f:              ok,
			want:           nil,
			wantErr:        false,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			f:              fail("1", "2", "3"),
			want:           []string{"1", "2", "3"},
			wantErr:        true,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"),
			f:              fail("1", "2", "6", "10"),
			want:           []string{"1", "2", "6", "10"},
			wantErr:        true,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9", "10"),
			f:              fail("1", "2", "6", "10"),
			want:           []string{"1", "2", "6", "10"},
			wantErr:        true,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"),
			f:              fail("1", "2", "6", "10"),
			want:           []string{"1", "2", "6", "10"},
			wantErr:        true,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19"),
			f:              fail("1", "2", "6", "10"),
			want:           []string{"1", "2", "6", "10"},
			wantErr:        true,
		},
		{
			goroutineCount: 3,
			eps:            eps("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28"),
			f:              fail("1", "2", "6", "10"),
			want:           []string{"1", "2", "6", "10"},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := pingAll(logger, tt.goroutineCount, tt.eps, tt.f)
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("pingAll() got %v, want %v", gotErr, tt.wantErr)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("pingAll() returned %v unreachable endpoints %v, want %v", len(got), got, tt.want)
			}

			for _, ep := range got {
				if !slices.Contains(tt.want, ep.Ip) {
					t.Fatalf("pingAll() got %v missing in want %v", ep.Ip, tt.want)
				}
			}
		})
	}
}

// The joined error of pingAll must satisfy errors.Is(err, ErrUnreachable) when
// the ping function reports unreachable nodes, as callers rely on it to
// distinguish unreachable nodes from failures of the pinging itself.
func TestPingAllErrorIsUnreachable(t *testing.T) {
	logger := zerolog.Logger{}

	f := func(logger zerolog.Logger, count int, ep NodeEndpoint) error {
		return fmt.Errorf("unhealthy connection: %w", ErrUnreachable)
	}

	_, err := pingAll(logger, 3, []NodeEndpoint{{Ip: "1"}, {Ip: "2"}}, f)
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("pingAll() = %v, want ErrUnreachable", err)
	}
}

// Generates a fresh ed25519 keypair, returning the private key PEM encoded as
// stored in [NodeEndpoint.SSHKey] and the corresponding public key.
func newTestClientKey(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("failed to marshal client key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to convert client public key: %v", err)
	}

	return string(pem.EncodeToMemory(block)), sshPub
}

// Starts an in-process SSH server on a random localhost port that accepts
// public key auth for the passed in key only. Returns the listen address.
func newTestSSHServer(t *testing.T, authorized ssh.PublicKey) string {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("failed to generate host key: %v", err)
	}

	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("failed to create host signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("unauthorized")
		},
	}
	cfg.AddHostKey(hostSigner)

	//nolint
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					conn.Close()
					return
				}
				go ssh.DiscardRequests(reqs)
				go func() {
					for ch := range chans {
						_ = ch.Reject(ssh.UnknownChannelType, "unsupported")
					}
				}()
				_ = sconn.Wait()
				_ = sconn.Close()
			}()
		}
	}()

	return l.Addr().String()
}

// Starts a server on a random localhost port that completes the TCP handshake
// but never sends a byte, mimicking a node whose kernel is alive but whose
// sshd is wedged. Returns the listen address.
func newSilentServer(t *testing.T) string {
	t.Helper()

	//nolint
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		// Hold on to the connections so they stay open, but never
		// respond on them.
		var conns []net.Conn
		for {
			conn, err := l.Accept()
			if err != nil {
				for _, c := range conns {
					c.Close()
				}
				return
			}
			conns = append(conns, conn)
		}
	}()

	return l.Addr().String()
}

func endpoint(t *testing.T, addr, sshKey string) NodeEndpoint {
	t.Helper()

	ip, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split address %q: %v", addr, err)
	}

	return NodeEndpoint{
		Ip:       ip,
		Port:     port,
		Username: "claudie",
		SSHKey:   sshKey,
	}
}

func TestSSHPing(t *testing.T) {
	if testing.Short() {
		t.Skipf("skipping ssh ping tests")
	}

	logger := zerolog.New(os.Stdout)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		key, pub := newTestClientKey(t)
		addr := newTestSSHServer(t, pub)

		if err := SSHPing(logger, PingRetryCount, endpoint(t, addr, key)); err != nil {
			t.Fatalf("SSHPing() = %v, want nil", err)
		}
	})

	t.Run("incomplete-endpoint", func(t *testing.T) {
		t.Parallel()

		err := SSHPing(logger, 1, NodeEndpoint{Ip: "127.0.0.1"})
		if !errors.Is(err, ErrUnreachable) {
			t.Fatalf("SSHPing() = %v, want ErrUnreachable", err)
		}
	})

	t.Run("closed-port", func(t *testing.T) {
		t.Parallel()

		key, _ := newTestClientKey(t)

		// Reserve a port and close it again so that nothing listens on it.
		//nolint
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		addr := l.Addr().String()
		l.Close()

		if err := SSHPing(logger, 1, endpoint(t, addr, key)); !errors.Is(err, ErrUnreachable) {
			t.Fatalf("SSHPing() = %v, want ErrUnreachable", err)
		}
	})

	// A node that is alive but refuses the presented key counts as
	// unreachable, an SSH ping verifies the node is manageable by
	// claudie, not just that the machine is up.
	t.Run("auth-rejected", func(t *testing.T) {
		t.Parallel()

		key, _ := newTestClientKey(t)
		_, otherPub := newTestClientKey(t)
		addr := newTestSSHServer(t, otherPub)

		if err := SSHPing(logger, 1, endpoint(t, addr, key)); !errors.Is(err, ErrUnreachable) {
			t.Fatalf("SSHPing() = %v, want ErrUnreachable", err)
		}
	})

	// A host that accepts the TCP connection but never responds must fail
	// within the handshake deadline instead of blocking forever, see the
	// SetDeadline call in sshPing.
	t.Run("silent-server-bounded", func(t *testing.T) {
		t.Parallel()

		key, _ := newTestClientKey(t)
		addr := newSilentServer(t)

		done := make(chan error, 1)
		go func() {
			done <- SSHPing(logger, 1, endpoint(t, addr, key))
		}()

		// One attempt costs at most the TCP dial + the handshake
		// deadline + the retry sleep, anything above that means the
		// handshake is not bounded.
		watchdog := 3*PingTimeout + 2*time.Second

		select {
		case err := <-done:
			if !errors.Is(err, ErrUnreachable) {
				t.Fatalf("SSHPing() = %v, want ErrUnreachable", err)
			}
		case <-time.After(watchdog):
			t.Fatalf("SSHPing() against a silent server did not return within %v, handshake deadline not enforced", watchdog)
		}
	})
}
