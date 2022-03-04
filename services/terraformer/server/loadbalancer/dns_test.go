package loadbalancer

import (
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/stretchr/testify/require"
)

var provider1 = &pb.Provider{
	Name:        "gcp",
	Credentials: "keys/platform-296509-d6ddeb344e91.json",
}

var provider2 = &pb.Provider{
	Name:        "gcp",
	Credentials: "keys/platform-infrastructure-316112-bd7953f712df.json",
}

var dns1 = DNS{
	CurrentProvider: provider1,
	DesiredProvider: provider2,
}
var dns2 = DNS{
	CurrentProvider: provider1,
	DesiredProvider: provider1,
}

func TestCheckDNSProvider(t *testing.T) {
	b1, err := dns1.checkDNSProvider()
	require.NoError(t, err)
	b2, err := dns2.checkDNSProvider()
	require.NoError(t, err)
	require.Equal(t, true, b1)
	require.Equal(t, false, b2)
}
