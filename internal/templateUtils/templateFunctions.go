package templateUtils

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/berops/claudie/proto/pb"
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

// ExtractNetmaskFromCIDR extracts the netmask from the CIDR notation.
func ExtractNetmaskFromCIDR(cidr string) string {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}

	ones, _ := n.Mask.Size()
	return fmt.Sprintf("%v", ones)
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
