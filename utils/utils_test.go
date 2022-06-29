package utils_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var provider1 = &pb.Provider{
	Name:        "gcp",
	Credentials: "keys/platform-296509-d6ddeb344e91.json",
}

var provider2 = &pb.Provider{
	Name:        "gcp",
	Credentials: "keys/platform-infrastructure-316112-bd7953f712df.json",
}

var dns1 = &pb.DNS{
	Provider: provider1,
}

var dns2 = &pb.DNS{
	Provider: provider2,
}

func TestCreateKeyFiles(t *testing.T) {
	private, public := makeSSHKeyPair()
	err := utils.CreateKeyFile(private, "", "private.pem")
	err1 := utils.CreateKeyFile(public, "", "public.pem")
	require.NoError(t, err, err1)
}

func TestCreateHash(t *testing.T) {
	hashLength := 0
	hash := utils.CreateHash(hashLength)
	fmt.Printf("Hash is %s\n", hash)
}

func makeSSHKeyPair() (string, string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2042)
	if err != nil {
		return "", ""
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", ""
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", ""
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return privKeyBuf.String(), pubKeyBuf.String()
}

func TestCheckDNSProvider(t *testing.T) {
	b1 := utils.ChangedDNSProvider(dns1, dns2)
	b2 := utils.ChangedDNSProvider(dns1, dns1)
	require.Equal(t, true, b1)
	require.Equal(t, false, b2)
}

func TestWrapperOut(t *testing.T) {
	command := "ls -la"
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = utils.GetStdOut("this is no error")
	cmd.Stderr = utils.GetStdErr("this is error")
	out, err := cmd.CombinedOutput()
	fmt.Println(out)
	require.NoError(t, err)
	cmd1 := exec.Command("bash", "-c", "dummy")
	cmd1.Stdout = utils.GetStdOut("this is no error")
	cmd1.Stderr = utils.GetStdErr("this is error")
	err = cmd1.Run()
	require.Error(t, err)
}

func TestCmd(t *testing.T) {
	command := "ls -la"
	cmd := utils.Cmd{Command: command, Stdout: utils.GetStdOut("this is no error"), Stderr: utils.GetStdErr("this is error")}
	err := cmd.RetryCommand(10)
	require.NoError(t, err)
}
