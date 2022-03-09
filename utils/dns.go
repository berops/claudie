package utils

import "github.com/Berops/platform/proto/pb"

func ChangedDNSProvider(currentDNS, desiredDNS *pb.DNS) bool {
	// DNS not yet created
	if currentDNS == nil {
		return false
	}
	// DNS provider are same
	if currentDNS.Provider.Name == desiredDNS.Provider.Name {
		if currentDNS.Provider.Credentials == desiredDNS.Provider.Credentials {
			return false
		}
	}
	return true
}

func ChangedAPIEndpoint(currentDNS, desiredDNS *pb.DNS) bool {
	if currentDNS == nil {
		return false
	}
	if currentDNS.Endpoint == desiredDNS.Endpoint {
		return false
	}
	return true
}
