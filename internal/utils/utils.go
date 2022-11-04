package utils

import (
	"fmt"
	"strings"
)

const (
	basePriority = 200
)

var (
	vmSizes = []string{"D3v3", "DS3v2", "D4v2", "DS4v2", "D5v2", "DS5v2", "D12v2", "DS12v2", "D13v2", "DS13v2", "D14v2", "DS14v2", "D15v2", "DS15v2", "F8", "FS8", "F16", "FS16", "M64s", "M64ms", "M128s", "M128ms", "D8", "D8Sv3", "D16", "D16Sv3", "D32", "D32Sv3", "D64", "D64Sv3", "E8", "E8Sv3", "E16", "E16Sv3", "E32", "E32Sv3", "E64", "E64Sv3"}
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

// EnableAccNet will check if accelerated networking can be enabled based on conditions
// specified here https://azure.microsoft.com/en-us/updates/accelerated-networking-in-expanded-preview/
func EnableAccNet(vmSize string) string {
	if !checkContains(vmSizes, vmSize) {
		return "false"
	}
	return "true"
}

func checkContains(arr []string, str string) bool {
	for _, el := range arr {
		if strings.Contains(str, el) {
			return true
		}
	}
	return false
}
