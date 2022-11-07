package utils

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	basePriority = 200
)

var (
	//regex of supported VM sizes
	vmSizes = []string{"(.D3.*?v3.*)", "(.DS3.*?v2.*)", "(.DS?4.*?v2..*)", "(.DS?5.*?v2.*)", "(.DS?12.*?v2.*)", "(.DS?13.*?v2.*)", "(.DS?14.*?v2.*)", "(.DS?15.*?v2.*)", "(.Fs?8.*)", "(.Fs?16.*)", "(.M64m?s.*)", "(.M128m?s.*)", "(.D8s?.*)", "(.D16s?.*)", "(.D32s?.*)", "(.D64s?.*)", "(.E8s?.*)", "(.E16s?.*)", "(.E32s?.*)", "(.E64s?.*)"}
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

// ProtocolNameToAzureProtocolString will check the protocol string and return one which is used
// in azure templates.
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
// we will look only at VM sizes, since all regions are supported now all reasonable operating systems
func EnableAccNet(vmSize string) string {
	if !checkContains(vmSizes, vmSize) {
		return "false"
	}
	return "true"
}

func checkContains(arr []string, str string) bool {
	for _, el := range arr {
		//if match and no error, return true
		if match, err := regexp.MatchString(el, str); err == nil && match {
			return true
		}
	}
	return false
}
