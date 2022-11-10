package templateUtils

import (
	"fmt"
	"net"
	"strings"

	"github.com/Berops/claudie/proto/pb"
)

const (
	basePriority = 200
)

// IsMissing checks if item is missing in the list of items.
func IsMissing[K comparable](item K, items []K) bool {
	for _, v := range items {
		if item == v {
			return false
		}
	}

	return true
}

// ProtocolNameToOCIProtocolNumber translates between a string version of a protocol
// to a number version that can be used within OCI. More info in the following link:
// https://docs.oracle.com/en-us/iaas/tools/terraform-provider-oci/4.96/docs/r/core_security_list.html
func ProtocolNameToOCIProtocolNumber(protocol string) int {
	// ICMP (“1”), TCP (“6”), UDP (“17”), and ICMPv6 (“58”).
	switch strings.ToLower(protocol) {
	case "tcp":
		return 6
	case "udp":
		return 17
	case "icmp":
		return 1
	case "icmpv6":
		return 58
	default:
		panic(fmt.Sprintf("unexpected protocol %s", protocol))
	}
}

// ExtractTargetPorts extracts target ports defined inside the role in the LoadBalancer.
func ExtractTargetPorts(loadBalancers []*pb.LBcluster) []int {
	ports := make(map[int32]struct{})

	var result []int
	for _, c := range loadBalancers {
		for _, role := range c.Roles {
			if _, ok := ports[role.TargetPort]; !ok {
				result = append(result, int(role.TargetPort))
			}
			ports[role.TargetPort] = struct{}{}
		}
	}

	return result
}

// ProtocolNameToAzureProtocolString returns string constants for transport protocols
func ProtocolNameToAzureProtocolString(protocol string) string {
	switch strings.ToLower(protocol) {
	case "tcp":
		return "Tcp"
	case "udp":
		return "Udp"
	case "icmp":
		return "Icmp"
	default:
		panic(fmt.Sprintf("unexpected protocol %s", protocol))
	}
}

// AssignPriority returns a priority for firewall rule with basePriority + index
func AssignPriority(index int) int {
	return basePriority + index
}

// GetCIDR function returns CIDR in IPv4 format, with position replaced by value
// The function does not check if it is a valid CIDR/can be used in subnet spec
// Example
// GetCIDR("10.0.0.0/8", 2, 1) will return "10.0.1.0/8"
// GetCIDR("10.0.0.0/8", 3, 1) will return "10.0.0.1/8"
func GetCIDR(baseCIDR string, position, value int) string {
	_, ipNet, err := net.ParseCIDR(baseCIDR)
	if err != nil {
		return fmt.Sprintf("Cannot parse a CIDR with base %s, position %d, value %d", baseCIDR, position, value)
	}
	ip := ipNet.IP
	ip[position] = byte(value)
	ones, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones)
}
