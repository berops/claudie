package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestGRPC_WatchForDoneOrErrorDocuments(t *testing.T) {
	type fields struct{ Store store.Store }
	type args struct{ ctx context.Context }

	tests := []struct {
		name     string
		fields   fields
		args     args
		wantErr  bool
		setup    func(db store.Store)
		validate func(t *testing.T, db store.Store)
	}{
		{
			name: "test-manifest-checksum-not-equal-done-stage",
			fields: fields{
				Store: func() store.Store {
					db := store.NewInMemoryStore()
					_ = db.CreateConfig(context.Background(), &store.Config{
						Version: 0,
						Name:    "checksum-not-equal-done-stage",
						Manifest: store.Manifest{
							Raw:                 "testing-manifest",
							Checksum:            hash.Digest("01"),
							LastAppliedChecksum: hash.Digest("02"),
						},
					})
					return db
				}(),
			},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-not-equal-done-stage")
				cfg.Manifest.State = manifest.Done.String()
				_ = db.UpdateConfig(context.Background(), cfg)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-not-equal-done-stage")
				assert.Equal(t, cfg.Manifest.State, manifest.Pending.String())
				assert.Equal(t, uint64(2), cfg.Version)
			},
		},
		{
			name: "test-manifest-checksum-not-equal-error-stage",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-not-equal-error-stage",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            hash.Digest("01"),
						LastAppliedChecksum: hash.Digest("02"),
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-not-equal-error-stage")
				cfg.Manifest.State = manifest.Error.String()
				_ = db.UpdateConfig(context.Background(), cfg)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-not-equal-error-stage")
				assert.Equal(t, cfg.Manifest.State, manifest.Pending.String())
				assert.Equal(t, uint64(2), cfg.Version)
			},
		},
		{
			name: "test-manifest-checksum-equal-done-stage",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-equal-done-stage",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            hash.Digest("01"),
						LastAppliedChecksum: hash.Digest("01"),
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-done-stage")
				cfg.Manifest.State = manifest.Done.String()
				_ = db.UpdateConfig(context.Background(), cfg)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-done-stage")
				assert.Equal(t, cfg.Manifest.State, manifest.Done.String())
				assert.Equal(t, uint64(1), cfg.Version)
			},
		},
		{
			name: "test-manifest-checksum-equal-error-stage",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-equal-error-stage",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            hash.Digest("01"),
						LastAppliedChecksum: hash.Digest("01"),
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-error-stage")
				cfg.Manifest.State = manifest.Error.String()
				_ = db.UpdateConfig(context.Background(), cfg)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-error-stage")
				assert.Equal(t, cfg.Manifest.State, manifest.Error.String())
				assert.Equal(t, uint64(1), cfg.Version)
			},
		},
		{
			name: "test-manifest-checksum-equal-done-stage-delete-config",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-equal-done-stage-delete-config",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            nil,
						LastAppliedChecksum: nil,
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-done-stage-delete-config")
				cfg.Manifest.State = manifest.Done.String()
				_ = db.UpdateConfig(context.Background(), cfg)
			},
			validate: func(t *testing.T, db store.Store) {
				_, err := db.GetConfig(context.Background(), "checksum-equal-done-stage-delete-config")
				assert.True(t, errors.Is(err, store.ErrNotFoundOrDirty))
				assert.Error(t, err)
			},
		},
		{
			name: "test-manifest-checksum-equal-done-stage-delete-clusters",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-equal-done-stage-delete-clusters",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            hash.Digest("0"),
						LastAppliedChecksum: hash.Digest("0"),
					},
					Clusters: map[string]*store.ClusterState{
						"test-cluster-1": {
							Current: store.Clusters{K8s: []byte("random")},
							Desired: store.Clusters{},
						},
						"test-cluster-2": {
							Current: store.Clusters{},
							Desired: store.Clusters{K8s: []byte("random")},
						},
						"test-cluster-3": {
							Current: store.Clusters{},
							Desired: store.Clusters{},
						},
						"test-cluster-4": {
							Current: store.Clusters{},
							Desired: store.Clusters{},
						},
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-done-stage-delete-clusters")
				cfg.Manifest.State = manifest.Done.String()
				_ = db.UpdateConfig(context.Background(), cfg)
				assert.NotNil(t, cfg.Clusters["test-cluster-3"])
				assert.NotNil(t, cfg.Clusters["test-cluster-4"])
				assert.NotNil(t, cfg.Clusters["test-cluster-1"])
				assert.NotNil(t, cfg.Clusters["test-cluster-2"])
				assert.Equal(t, uint64(1), cfg.Version)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-done-stage-delete-clusters")
				assert.Nil(t, cfg.Clusters["test-cluster-3"])
				assert.Nil(t, cfg.Clusters["test-cluster-4"])
				assert.NotNil(t, cfg.Clusters["test-cluster-1"])
				assert.NotNil(t, cfg.Clusters["test-cluster-2"])
				assert.Equal(t, uint64(2), cfg.Version)
			},
		},
		{
			name: "test-manifest-checksum-equal-error-stage-delete-clusters",
			fields: fields{Store: func() store.Store {
				db := store.NewInMemoryStore()
				_ = db.CreateConfig(context.Background(), &store.Config{
					Version: 0,
					Name:    "checksum-equal-error-stage-delete-clusters",
					Manifest: store.Manifest{
						Raw:                 "testing-manifest",
						Checksum:            hash.Digest("0"),
						LastAppliedChecksum: hash.Digest("0"),
					},
					Clusters: map[string]*store.ClusterState{
						"test-cluster-1": {
							Current: store.Clusters{K8s: []byte("")},
							Desired: store.Clusters{},
						},
						"test-cluster-2": {
							Current: store.Clusters{},
							Desired: store.Clusters{K8s: []byte("")},
						},
						"test-cluster-3": {
							Current: store.Clusters{},
							Desired: store.Clusters{},
						},
						"test-cluster-4": {
							Current: store.Clusters{},
							Desired: store.Clusters{},
						},
					},
				})
				return db
			}()},
			args:    args{ctx: context.Background()},
			wantErr: false,
			setup: func(db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-error-stage-delete-clusters")
				cfg.Manifest.State = manifest.Error.String()
				_ = db.UpdateConfig(context.Background(), cfg)
				assert.NotNil(t, cfg.Clusters["test-cluster-3"])
				assert.NotNil(t, cfg.Clusters["test-cluster-4"])
				assert.NotNil(t, cfg.Clusters["test-cluster-1"])
				assert.NotNil(t, cfg.Clusters["test-cluster-2"])
				assert.Equal(t, uint64(1), cfg.Version)
			},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "checksum-equal-error-stage-delete-clusters")
				assert.NotNil(t, cfg.Clusters["test-cluster-3"])
				assert.NotNil(t, cfg.Clusters["test-cluster-4"])
				assert.NotNil(t, cfg.Clusters["test-cluster-1"])
				assert.NotNil(t, cfg.Clusters["test-cluster-2"])
				assert.Equal(t, uint64(1), cfg.Version)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.setup(tt.fields.Store)

			g := &GRPC{Store: tt.fields.Store}

			if err := g.WatchForDoneOrErrorDocuments(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("WatchForDoneOrErrorDocuments() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.validate(t, tt.fields.Store)
		})
	}
}

func TestGRPC_WatchForPendingDocuments(t *testing.T) {
	db := store.NewInMemoryStore()
	_ = db.CreateConfig(context.Background(), &store.Config{
		Version: 0,
		Name:    "Test-set1",
		Manifest: store.Manifest{
			Raw: `
name: Test-set1
providers:
  hetzner:
    - name: hetzner-1
      credentials: credentials
      templates:
        repository: "https://github.com/berops/claudie-config"
        path: "templates/terraformer/hetzner"
nodePools:
  dynamic:
    - name: htz-ctrl-nodes
      providerSpec:
        name: hetzner-1
        region: nbg1
        zone: nbg1-dc3
      count: 1
      serverType: cpx11
      image: ubuntu-24.04
kubernetes:
  clusters:
    - name: test-set-1
      version: 1.27.0
      network: 192.168.2.0/24
      pools:
        control:
          - htz-ctrl-nodes
        compute:
          - htz-ctrl-nodes
`,
			Checksum: hash.Digest("random-checksum"),
		},
	})

	type fields struct{ Store store.Store }
	type args struct{ ctx context.Context }

	tests := []struct {
		name     string
		fields   fields
		args     args
		wantErr  bool
		validate func(t *testing.T, db store.Store)
	}{
		{
			name:    "ok-desired-state-and-tasks",
			fields:  fields{Store: db},
			args:    args{ctx: context.Background()},
			wantErr: false,
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "Test-set1")
				assert.Equal(t, manifest.Scheduled.String(), cfg.Manifest.State)
				assert.Equal(t, uint64(1), cfg.Version)
				assert.NotNil(t, cfg.Clusters)
				assert.NotNil(t, cfg.Clusters["test-set-1"])
				assert.NotNil(t, cfg.Clusters["test-set-1"].Events.TaskEvents)
				assert.Equal(t, int(1), len(cfg.Clusters["test-set-1"].Events.TaskEvents))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &GRPC{Store: tt.fields.Store}

			if err := g.WatchForPendingDocuments(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("WatchForPendingDocuments() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.validate(t, tt.fields.Store)
		})
	}
}

func TestGRPC_WatchForScheduledDocuments(t *testing.T) {
	type fields struct{ Store store.Store }
	type args struct{ ctx context.Context }

	tests := []struct {
		name     string
		fields   fields
		args     args
		wantErr  bool
		validate func(t *testing.T, db store.Store)
	}{
		{
			name: "test-config-ends-in-scheduled",
			fields: fields{
				Store: func() store.Store {
					db := store.NewInMemoryStore()
					_ = db.CreateConfig(context.Background(), &store.Config{
						Version:  0,
						Name:     "test-set-1",
						K8SCtx:   store.KubernetesContext{},
						Manifest: store.Manifest{},
						Clusters: map[string]*store.ClusterState{
							"test-cluster-1": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events: store.Events{TaskEvents: []store.TaskEvent{
									{Id: "3", Timestamp: time.Now().UTC().Format(time.RFC3339)},
									{Id: "4", Timestamp: time.Now().UTC().Format(time.RFC3339)},
								}, TTL: 40},
								State: store.Workflow{Status: spec.Workflow_ERROR.String()},
							},
							"test-cluster-2": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events: store.Events{TaskEvents: []store.TaskEvent{
									{Id: "1", Timestamp: time.Now().UTC().Format(time.RFC3339)},
									{Id: "2", Timestamp: time.Now().UTC().Format(time.RFC3339)},
								}, TTL: 10},
								State: store.Workflow{Status: spec.Workflow_DONE.String()},
							},
						},
					})
					cfg, _ := db.GetConfig(context.Background(), "test-set-1")
					cfg.Manifest.State = manifest.Scheduled.String()
					_ = db.UpdateConfig(context.Background(), cfg)
					return db
				}(),
			},
			args: args{ctx: context.Background()},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "test-set-1")
				assert.Equal(t, manifest.Scheduled.String(), cfg.Manifest.State)
				assert.Equal(t, uint64(2), cfg.Version)
				assert.Equal(t, int32(9), cfg.Clusters["test-cluster-2"].Events.TTL)
				assert.Equal(t, int32(40), cfg.Clusters["test-cluster-1"].Events.TTL)
			},
			wantErr: false,
		},
		{
			name: "test-config-ends-in-scheduled",
			fields: fields{
				Store: func() store.Store {
					db := store.NewInMemoryStore()
					_ = db.CreateConfig(context.Background(), &store.Config{
						Version:  0,
						Name:     "test-set-1",
						K8SCtx:   store.KubernetesContext{},
						Manifest: store.Manifest{},
						Clusters: map[string]*store.ClusterState{
							"test-cluster-1": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events: store.Events{TaskEvents: []store.TaskEvent{
									{Id: "3", Timestamp: time.Now().UTC().Format(time.RFC3339)},
									{Id: "4", Timestamp: time.Now().UTC().Format(time.RFC3339)},
								}, TTL: 40},
								State: store.Workflow{Status: spec.Workflow_ERROR.String()},
							},
							"test-cluster-2": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events: store.Events{TaskEvents: []store.TaskEvent{
									{Id: "1", Timestamp: time.Now().UTC().Format(time.RFC3339)},
									{Id: "2", Timestamp: time.Now().UTC().Format(time.RFC3339)},
								}, TTL: 10},
								State: store.Workflow{Status: spec.Workflow_ERROR.String()},
							},
						},
					})
					cfg, _ := db.GetConfig(context.Background(), "test-set-1")
					cfg.Manifest.State = manifest.Scheduled.String()
					_ = db.UpdateConfig(context.Background(), cfg)
					return db
				}(),
			},
			args: args{ctx: context.Background()},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "test-set-1")
				assert.Equal(t, manifest.Error.String(), cfg.Manifest.State)
				assert.Equal(t, uint64(2), cfg.Version)
				assert.Equal(t, int32(10), cfg.Clusters["test-cluster-2"].Events.TTL)
				assert.Equal(t, int32(40), cfg.Clusters["test-cluster-1"].Events.TTL)
			},
			wantErr: false,
		},
		{
			name: "test-config-ends-in-scheduled",
			fields: fields{
				Store: func() store.Store {
					db := store.NewInMemoryStore()
					_ = db.CreateConfig(context.Background(), &store.Config{
						Version:  0,
						Name:     "test-set-1",
						K8SCtx:   store.KubernetesContext{},
						Manifest: store.Manifest{},
						Clusters: map[string]*store.ClusterState{
							"test-cluster-1": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events:  store.Events{TaskEvents: []store.TaskEvent{}, TTL: 0},
								State:   store.Workflow{Status: spec.Workflow_DONE.String()},
							},
							"test-cluster-2": {
								Current: store.Clusters{},
								Desired: store.Clusters{},
								Events:  store.Events{TaskEvents: []store.TaskEvent{}, TTL: 0},
								State:   store.Workflow{Status: spec.Workflow_DONE.String()},
							},
						},
					})
					cfg, _ := db.GetConfig(context.Background(), "test-set-1")
					cfg.Manifest.State = manifest.Scheduled.String()
					_ = db.UpdateConfig(context.Background(), cfg)
					return db
				}(),
			},
			args: args{ctx: context.Background()},
			validate: func(t *testing.T, db store.Store) {
				cfg, _ := db.GetConfig(context.Background(), "test-set-1")
				assert.Equal(t, manifest.Done.String(), cfg.Manifest.State)
				assert.Equal(t, uint64(2), cfg.Version)
				assert.Equal(t, int32(0), cfg.Clusters["test-cluster-2"].Events.TTL)
				assert.Equal(t, int32(0), cfg.Clusters["test-cluster-1"].Events.TTL)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &GRPC{Store: tt.fields.Store}
			if err := g.WatchForScheduledDocuments(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("WatchForScheduledDocuments() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.validate(t, tt.fields.Store)
		})
	}
}
