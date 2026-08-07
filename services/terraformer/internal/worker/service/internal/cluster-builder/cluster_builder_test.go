package cluster_builder

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func Test_parseProviderVersions(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "provider_version.tpl")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("single provider", func(t *testing.T) {
		got, err := parseProviderVersions(write(t, `
terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.60.0"
    }
  }
}`))
		if err != nil {
			t.Fatal(err)
		}
		want := ProviderBlock{Source: "hetznercloud/hcloud", Version: "~> 1.60.0"}
		if len(got) != 1 || got["hcloud"] != want {
			t.Errorf("parseProviderVersions() = %v, want map[hcloud:%v]", got, want)
		}
	})

	t.Run("multiple providers and extra terraform settings", func(t *testing.T) {
		got, err := parseProviderVersions(write(t, `
terraform {
  required_version = ">= 1.0"
  required_providers {
    verda = {
      source  = "verda-cloud/verda"
      version = "~> 1.1"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
  }
}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got["verda"].Version != "~> 1.1" || got["http"].Version != "~> 3.4" {
			t.Errorf("parseProviderVersions() = %v", got)
		}
	})

	errCases := []struct {
		name    string
		content string
	}{
		{name: "empty required_providers", content: `
terraform {
  required_providers {
  }
}`},
		{name: "missing terraform block", content: `locals {}`},
		{name: "missing version", content: `
terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
    }
  }
}`},
		{name: "empty version", content: `
terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = ""
    }
  }
}`},
		{name: "invalid version constraint", content: `
terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "latest"
    }
  }
}`},
	}
	for _, tt := range errCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseProviderVersions(write(t, tt.content)); err == nil {
				t.Error("parseProviderVersions() expected error, got nil")
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		if _, err := parseProviderVersions(filepath.Join(t.TempDir(), "provider_version.tpl")); err == nil {
			t.Error("parseProviderVersions() expected error, got nil")
		}
	})
}

func Test_lowerBound(t *testing.T) {
	tests := []struct {
		constraint string
		want       string
		wantErr    bool
	}{
		{constraint: "6.44.0", want: "6.44.0"},
		{constraint: "= 1.5", want: "1.5.0"},
		{constraint: "~> 1.60", want: "1.60.0"},
		{constraint: "~> 1.60.0", want: "1.60.0"},
		{constraint: ">=1.2", want: "1.2.0"},
		{constraint: ">= 1.2, < 2.0", want: "1.2.0"},
		{constraint: "> 1.0, != 1.5", want: "1.0.0"},
		{constraint: "< 2.0", wantErr: true},
		{constraint: "!= 1.5", wantErr: true},
		{constraint: "latest", wantErr: true},
		{constraint: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			got, err := lowerBound(tt.constraint)
			if (err != nil) != tt.wantErr {
				t.Fatalf("lowerBound(%q) error = %v, wantErr %v", tt.constraint, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.String() != tt.want {
				t.Errorf("lowerBound(%q) = %q, want %q", tt.constraint, got, tt.want)
			}
		})
	}
}

func Test_mergeProviderVersions(t *testing.T) {
	hcloud := func(v string) map[string]ProviderBlock {
		return map[string]ProviderBlock{"hcloud": {Source: "hetznercloud/hcloud", Version: v}}
	}
	// merge merges a then b into an empty map, failing the test on error.
	merge := func(t *testing.T, a, b map[string]ProviderBlock) map[string]ProviderBlock {
		t.Helper()
		dst := map[string]ProviderBlock{}
		for _, src := range []map[string]ProviderBlock{a, b} {
			if err := mergeProviderVersions(dst, src, zerolog.Nop()); err != nil {
				t.Fatal(err)
			}
		}
		return dst
	}

	tests := []struct {
		name string
		a, b map[string]ProviderBlock
		want string
	}{
		{name: "higher pessimistic constraint wins", a: hcloud("~> 1.45"), b: hcloud("~> 1.60.0"), want: "~> 1.60.0"},
		{name: "higher exact version wins over range", a: hcloud("~> 6.40"), b: hcloud("6.44.0"), want: "6.44.0"},
		{name: "equal versions are kept", a: hcloud("~> 1.60.0"), b: hcloud("~> 1.60.0"), want: "~> 1.60.0"},
		{name: "equal lower bounds tie-break deterministically", a: hcloud("~> 1.60"), b: hcloud(">= 1.60.0"), want: "~> 1.60"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The nodepool groups whose requirements are merged are iterated in
			// non-deterministic order, the result must not depend on it.
			if got := merge(t, tt.a, tt.b)["hcloud"].Version; got != tt.want {
				t.Errorf("merge(a, b) = %q, want %q", got, tt.want)
			}
			if got := merge(t, tt.b, tt.a)["hcloud"].Version; got != tt.want {
				t.Errorf("merge(b, a) = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("disjoint providers are unioned", func(t *testing.T) {
		got := merge(t, hcloud("~> 1.60.0"), map[string]ProviderBlock{"http": {Source: "hashicorp/http", Version: "~> 3.4"}})
		if len(got) != 2 {
			t.Errorf("merge() = %v, want hcloud and http", got)
		}
	})

	t.Run("conflicting sources error", func(t *testing.T) {
		dst := hcloud("~> 1.60.0")
		src := map[string]ProviderBlock{"hcloud": {Source: "someone-else/hcloud", Version: "~> 1.60.0"}}
		if err := mergeProviderVersions(dst, src, zerolog.Nop()); err == nil {
			t.Error("mergeProviderVersions() expected error, got nil")
		}
	})
}

func Test_generateCommonProviderVersions(t *testing.T) {
	var c ClusterBuilder
	c.inner.networkingDir = t.TempDir()

	want := map[string]ProviderBlock{
		"hcloud": {Source: "hetznercloud/hcloud", Version: "~> 1.60.0"},
		"http":   {Source: "hashicorp/http", Version: "~> 3.4"},
	}
	f := filepath.Join(c.inner.networkingDir, providerVersionFileName)
	if err := generateProviderVersions(f, want); err != nil {
		t.Fatal(err)
	}

	// The generated file must round-trip through the same parser that reads
	// the provider_version.tpl files.
	got, err := parseProviderVersions(f)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got, want) {
		t.Errorf("parseProviderVersions(generated file) = %v, want %v", got, want)
	}
}

func Test_parseNodeOutput(t *testing.T) {
	tests := []struct {
		name        string
		val         any
		wantIP      string
		wantSSHPort int32
		wantWGPort  int32
		wantErr     bool
	}{
		{name: "legacy string IP", val: "1.2.3.4", wantIP: "1.2.3.4"},
		// JSON numbers decode to float64, so mimic that for the array cases.
		{name: "ip + ssh port (numbers)", val: []any{"1.2.3.4", float64(22522)}, wantIP: "1.2.3.4", wantSSHPort: 22522},
		{name: "ip + ssh + wg ports (numbers)", val: []any{"1.2.3.4", float64(22222), float64(41234)}, wantIP: "1.2.3.4", wantSSHPort: 22222, wantWGPort: 41234},
		// CloudRift template emits ports via tostring(), so they arrive as strings.
		{name: "ip + ssh + wg ports (strings)", val: []any{"1.2.3.4", "22222", "41234"}, wantIP: "1.2.3.4", wantSSHPort: 22222, wantWGPort: 41234},
		{name: "ip only array", val: []any{"1.2.3.4"}, wantIP: "1.2.3.4"},
		{name: "zero/invalid ports fall back to 0", val: []any{"1.2.3.4", float64(0), "notaport"}, wantIP: "1.2.3.4"},
		{name: "port with suffix is rejected", val: []any{"1.2.3.4", "22222x", "41234x"}, wantIP: "1.2.3.4"},
		{name: "out-of-range ports fall back to 0", val: []any{"1.2.3.4", float64(65536), float64(99999)}, wantIP: "1.2.3.4"},
		{name: "non-string ip element errors", val: []any{float64(1234), "22222"}, wantErr: true},
		{name: "empty array errors", val: []any{}, wantErr: true},
		{name: "nil errors", val: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, sshPort, wgPort, err := parseNodeOutput(tt.val)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseNodeOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ip != tt.wantIP || sshPort != tt.wantSSHPort || wgPort != tt.wantWGPort {
				t.Errorf("parseNodeOutput() = (%q, %d, %d), want (%q, %d, %d)",
					ip, sshPort, wgPort, tt.wantIP, tt.wantSSHPort, tt.wantWGPort)
			}
		})
	}
}
