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

// generateSSHKeys will generate SSH keypair for each nodepool that does not yet have
// a keypair assigned.
func generateSSHKeys(desiredInfo *pb.ClusterInfo) error {
	for i := range desiredInfo.NodePools {
		if dp := desiredInfo.NodePools[i].GetDynamicNodePool(); dp != nil && dp.PublicKey == "" {
			var err error
			if dp.PublicKey, dp.PrivateKey, err = generateSSHKeyPair(); err != nil {
				return fmt.Errorf("error while create SSH key pair for nodepool %s: %w", desiredInfo.NodePools[i].Name, err)
			}
		}
	}
	return nil
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// Generate and write public key
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pubKey))

	return pubKeyBuf.String(), privKeyBuf.String(), nil
}
