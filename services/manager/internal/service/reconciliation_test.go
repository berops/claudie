package service

import (
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
)

func dynamicNodePool(provider *spec.Provider) *spec.NodePool {
	return &spec.NodePool{
		Type: &spec.NodePool_DynamicNodePool{
			DynamicNodePool: &spec.DynamicNodePool{
				Provider: provider,
			},
		},
	}
}

func staticNodePool(keys map[string]string) *spec.NodePool {
	return &spec.NodePool{
		Type: &spec.NodePool_StaticNodePool{
			StaticNodePool: &spec.StaticNodePool{
				NodeKeys: keys,
			},
		},
	}
}

func awsProvider(accessKey, secretKey string) *spec.Provider {
	return &spec.Provider{
		ProviderType: &spec.Provider_Aws{
			Aws: &spec.AWSProvider{AccessKey: accessKey, SecretKey: secretKey},
		},
	}
}

// nolint
func TestUpdateNodePoolCredentials(t *testing.T) {
	tests := []struct {
		name        string
		current     *spec.NodePool
		desired     *spec.NodePool
		wantUpdated bool
		assertFunc  func(t *testing.T, current *spec.NodePool)
	}{
		{
			name:        "dynamic: credentials differ — copies and returns updated=true",
			current:     dynamicNodePool(awsProvider("old-key", "old-secret")),
			desired:     dynamicNodePool(awsProvider("new-key", "new-secret")),
			wantUpdated: true,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				aws := current.Type.(*spec.NodePool_DynamicNodePool).DynamicNodePool.Provider.ProviderType.(*spec.Provider_Aws).Aws
				if aws.AccessKey != "new-key" {
					t.Errorf("AccessKey = %q, want %q", aws.AccessKey, "new-key")
				}
				if aws.SecretKey != "new-secret" {
					t.Errorf("SecretKey = %q, want %q", aws.SecretKey, "new-secret")
				}
			},
		},
		{
			name:        "dynamic: credentials equal — no copy, returns updated=false",
			current:     dynamicNodePool(awsProvider("same-key", "same-secret")),
			desired:     dynamicNodePool(awsProvider("same-key", "same-secret")),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				aws := current.Type.(*spec.NodePool_DynamicNodePool).DynamicNodePool.Provider.ProviderType.(*spec.Provider_Aws).Aws
				if aws.AccessKey != "same-key" {
					t.Errorf("AccessKey = %q, want %q", aws.AccessKey, "same-key")
				}
				if aws.SecretKey != "same-secret" {
					t.Errorf("SecretKey = %q, want %q", aws.SecretKey, "same-secret")
				}
			},
		},
		{
			name:        "dynamic: both providers have empty credentials — no update",
			current:     dynamicNodePool(awsProvider("", "")),
			desired:     dynamicNodePool(awsProvider("", "")),
			wantUpdated: false,
			assertFunc:  nil,
		},
		{
			name:        "dynamic: current has empty credentials, desired has values — copies, updated=true",
			current:     dynamicNodePool(awsProvider("", "")),
			desired:     dynamicNodePool(awsProvider("new-key", "new-secret")),
			wantUpdated: true,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				aws := current.Type.(*spec.NodePool_DynamicNodePool).DynamicNodePool.Provider.ProviderType.(*spec.Provider_Aws).Aws
				if aws.AccessKey != "new-key" {
					t.Errorf("AccessKey = %q, want %q", aws.AccessKey, "new-key")
				}
			},
		},
		{
			name:        "dynamic: current has credentials, desired has empty — clears credentials, updated=true",
			current:     dynamicNodePool(awsProvider("old-key", "old-secret")),
			desired:     dynamicNodePool(awsProvider("", "")),
			wantUpdated: true,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				aws := current.Type.(*spec.NodePool_DynamicNodePool).DynamicNodePool.Provider.ProviderType.(*spec.Provider_Aws).Aws
				if aws.AccessKey != "" {
					t.Errorf("AccessKey = %q, want empty string", aws.AccessKey)
				}
				if aws.SecretKey != "" {
					t.Errorf("SecretKey = %q, want empty string", aws.SecretKey)
				}
			},
		},
		{
			name:        "dynamic current, static desired — type mismatch, no update",
			current:     dynamicNodePool(awsProvider("old-key", "old-secret")),
			desired:     staticNodePool(map[string]string{"10.0.0.1": "ssh-key"}),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				aws := current.Type.(*spec.NodePool_DynamicNodePool).DynamicNodePool.Provider.ProviderType.(*spec.Provider_Aws).Aws
				if aws.AccessKey != "old-key" {
					t.Errorf("AccessKey = %q, want %q (receiver should be unchanged)", aws.AccessKey, "old-key")
				}
			},
		},
		{
			name: "static: one key differs — updates that key, returns updated=true",
			current: staticNodePool(map[string]string{
				"10.0.0.1": "old-key",
				"10.0.0.2": "unchanged-key",
			}),
			desired: staticNodePool(map[string]string{
				"10.0.0.1": "new-key",
				"10.0.0.2": "unchanged-key",
			}),
			wantUpdated: true,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if keys["10.0.0.1"] != "new-key" {
					t.Errorf("NodeKeys[10.0.0.1] = %q, want %q", keys["10.0.0.1"], "new-key")
				}
				if keys["10.0.0.2"] != "unchanged-key" {
					t.Errorf("NodeKeys[10.0.0.2] = %q, want %q", keys["10.0.0.2"], "unchanged-key")
				}
			},
		},
		{
			name: "static: all keys differ — updates all, returns updated=true",
			current: staticNodePool(map[string]string{
				"10.0.0.1": "old-key-1",
				"10.0.0.2": "old-key-2",
			}),
			desired: staticNodePool(map[string]string{
				"10.0.0.1": "new-key-1",
				"10.0.0.2": "new-key-2",
			}),
			wantUpdated: true,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if keys["10.0.0.1"] != "new-key-1" {
					t.Errorf("NodeKeys[10.0.0.1] = %q, want %q", keys["10.0.0.1"], "new-key-1")
				}
				if keys["10.0.0.2"] != "new-key-2" {
					t.Errorf("NodeKeys[10.0.0.2] = %q, want %q", keys["10.0.0.2"], "new-key-2")
				}
			},
		},
		{
			name: "static: all keys equal — no update, returns updated=false",
			current: staticNodePool(map[string]string{
				"10.0.0.1": "same-key",
			}),
			desired: staticNodePool(map[string]string{
				"10.0.0.1": "same-key",
			}),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if keys["10.0.0.1"] != "same-key" {
					t.Errorf("NodeKeys[10.0.0.1] = %q, want %q", keys["10.0.0.1"], "same-key")
				}
			},
		},
		{
			name: "static: endpoint absent in desired — current key preserved, no update",
			current: staticNodePool(map[string]string{
				"10.0.0.1": "key-1",
				"10.0.0.2": "key-2",
			}),
			desired: staticNodePool(map[string]string{
				"10.0.0.1": "key-1",
				// "10.0.0.2" intentionally missing
			}),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if keys["10.0.0.2"] != "key-2" {
					t.Errorf("NodeKeys[10.0.0.2] = %q, want %q (should be unchanged)", keys["10.0.0.2"], "key-2")
				}
			},
		},
		{
			name: "static: endpoint only in desired — not added to current, no update",
			current: staticNodePool(map[string]string{
				"10.0.0.1": "key-1",
			}),
			desired: staticNodePool(map[string]string{
				"10.0.0.1": "key-1",
				"10.0.0.2": "new-key",
			}),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if _, exists := keys["10.0.0.2"]; exists {
					t.Errorf("NodeKeys[10.0.0.2] should not exist in current after update")
				}
			},
		},
		{
			name:        "static: both have empty NodeKeys — no update",
			current:     staticNodePool(map[string]string{}),
			desired:     staticNodePool(map[string]string{}),
			wantUpdated: false,
			assertFunc:  nil,
		},
		{
			name:        "static: current has empty NodeKeys, desired has entries — no update (nothing to iterate)",
			current:     staticNodePool(map[string]string{}),
			desired:     staticNodePool(map[string]string{"10.0.0.1": "key"}),
			wantUpdated: false,
			assertFunc:  nil,
		},
		{
			name:        "static current, dynamic desired — type mismatch, no update",
			current:     staticNodePool(map[string]string{"10.0.0.1": "old-key"}),
			desired:     dynamicNodePool(awsProvider("key", "secret")),
			wantUpdated: false,
			assertFunc: func(t *testing.T, current *spec.NodePool) {
				keys := current.Type.(*spec.NodePool_StaticNodePool).StaticNodePool.NodeKeys
				if keys["10.0.0.1"] != "old-key" {
					t.Errorf("NodeKeys[10.0.0.1] = %q, want %q (should be unchanged)", keys["10.0.0.1"], "old-key")
				}
			},
		},
		{
			name:        "unknown nodepool type — no update, no panic",
			current:     &spec.NodePool{Type: nil},
			desired:     &spec.NodePool{Type: nil},
			wantUpdated: false,
			assertFunc:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := updateNodePoolCredentials(tc.current, tc.desired)
			if got != tc.wantUpdated {
				t.Errorf("updateNodePoolCredentials() updated = %v, want %v", got, tc.wantUpdated)
			}
			if tc.assertFunc != nil {
				tc.assertFunc(t, tc.current)
			}
		})
	}
}
