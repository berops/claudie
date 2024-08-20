package store

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/berops/claudie/internal/manifest"
)

var _ Store = (*InMemoryStore)(nil)

type InMemoryStore struct {
	db *sync.Map
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		db: new(sync.Map),
	}
}

func (i *InMemoryStore) Close() error { return nil }

func (i *InMemoryStore) HealthCheck() error { return nil }

func (i *InMemoryStore) CreateConfig(ctx context.Context, config *Config) error {
	if _, exists := i.db.Load(config.Name); exists {
		return fmt.Errorf("config %q already exists", config.Name)
	}

	config.Version = 0
	config.Manifest.State = manifest.Pending.String()
	i.db.Store(config.Name, config)

	return nil
}

func (i *InMemoryStore) UpdateConfig(ctx context.Context, config *Config) error {
	existing, exists := i.db.Load(config.Name)
	cfg := existing.(*Config)
	if !exists || cfg.Version != config.Version {
		return ErrNotFoundOrDirty
	}

	config.Version += 1
	i.db.Store(config.Name, config)

	return nil
}

func (i *InMemoryStore) GetConfig(ctx context.Context, name string) (*Config, error) {
	cfg, exists := i.db.Load(name)
	if !exists {
		return nil, ErrNotFoundOrDirty
	}
	return cfg.(*Config), nil
}

func (i *InMemoryStore) ListConfigs(ctx context.Context, filter *ListFilter) ([]*Config, error) {
	var out []*Config
	i.db.Range(func(key, value any) bool {
		cfg := value.(*Config)
		if !slices.Contains(filter.ManifestState, cfg.Manifest.State) {
			return true
		}
		out = append(out, cfg)
		return true
	})
	return out, nil
}

func (i *InMemoryStore) DeleteConfig(ctx context.Context, name string, version uint64) error {
	if cfg, exists := i.db.Load(name); !exists || cfg.(*Config).Version != version {
		return ErrNotFoundOrDirty
	}
	i.db.Delete(name)
	return nil
}

func (i *InMemoryStore) MarkForDeletion(ctx context.Context, name string, version uint64) error {
	existing, exists := i.db.Load(name)
	cfg := existing.(*Config)
	if !exists || cfg.Version != version {
		return ErrNotFoundOrDirty
	}

	cfg.Manifest.Raw = ""
	cfg.Manifest.Checksum = nil

	cfg.Version += 1

	i.db.Store(name, existing)

	return nil
}
