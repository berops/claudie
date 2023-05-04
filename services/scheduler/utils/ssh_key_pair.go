package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/berops/claudie/proto/pb"
)

// sshKeyPair is a struct containing private and public SSH keys as a string.
// These SSH key-pairs are required to SSH into the VMs in the cluster and execute commands.
type sshKeyPair struct {
	public  string
	private string
}

// createSSHKeyPair will create a RSA key-pair and save it into the clusterInfo provided
// return error if key pair creation fails
func createSSHKeyPair(desiredInfo *pb.ClusterInfo) error {
	// If the cluster doesn't have an SSH keypair associated with it, then generate a new SSH keypair and associate
	// with the cluster
	if desiredInfo.PublicKey == "" {
		keys, err := generateSSHKeyPair()
		if err != nil {
			return fmt.Errorf("error while creating SSH key pair for %s : %w", desiredInfo.Name, err)
		}

		desiredInfo.PrivateKey = keys.private
		desiredInfo.PublicKey = keys.public
	}
	return nil
}

// generateSSHKeyPair function generates SSH key pair
// returns the keypair if successful, nil otherwise
func generateSSHKeyPair() (sshKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return sshKeyPair{}, err
	}

	// Generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return sshKeyPair{}, err
	}

	// Generate and write public key
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return sshKeyPair{}, err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pubKey))

	return sshKeyPair{public: pubKeyBuf.String(), private: privKeyBuf.String()}, nil
}
