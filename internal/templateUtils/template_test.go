package templateUtils

import "testing"

func TestGetCIDR(t *testing.T) {
	cidr := GetCIDR("10.0.0.0/16", 2, 1)
	t.Log(cidr)
}
