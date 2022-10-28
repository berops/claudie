package utils

import (
	"fmt"
	"strings"
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

func AssignPriority(index int) int {
	return basePriority + index
}
