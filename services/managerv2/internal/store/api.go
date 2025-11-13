package store

import (
	"context"
	"errors"
	"io"

	"github.com/berops/claudie/internal/healthcheck"
)

// ErrNotFoundOrDirty is returned when the requested document couldn't be found inside the database or a Dirty Write occurred.
var ErrNotFoundOrDirty = errors.New("failed to find requested document. It is possible that this operation was a Dirty Write. Consider fetching the latest version of the requested document to repeat the read-write cycle")

// ListFilter wraps supported filters for listing configs.
type ListFilter struct {
	ManifestState []string
}

type Store interface {
	io.Closer
	healthcheck.HealthChecker

	// CRUD

	// CreateConfig creates a new config. It is up to the application logic to determine if the
	// config already exists or not. On conflict, the creation will error out. The Version field
	// of the config will always be overwritten to 0 as new configs always start with a version of 0.
	CreateConfig(ctx context.Context, config *Config) error

	// UpdateConfig updates an existing config in the database with the new supplied data. If there is no document
	// that matches the Config.Name and Config.Version the ErrNotFoundOrDirty err is returned. It is up to the application
	// code to determine, if the write was Dirty (i.e. outdated Document version used) or there is no such document with the
	// requested Config.Name and Config.Version combination. Before updating the document, a higher document version
	// number by 1 will replace the supplied number in Config.Version (i.e. Config.Version += 1).
	UpdateConfig(ctx context.Context, config *Config) error

	// GetConfig queries the document with the Config.Name. If no such document is found the ErrNotFoundOrDirty err
	// is returned. In this case it always will be the case that the document is absent. This can be used by the application
	// code to determine an absent document or Dirty Write.
	GetConfig(ctx context.Context, name string) (*Config, error)

	// ListConfigs queries all documents stored that satisfy the passed in ListFilter.
	ListConfigs(ctx context.Context, filter *ListFilter) ([]*Config, error)

	// DeleteConfig will delete the document with the requested Config.Name and Config.Version combination.
	// If No documents with the given combination were deleted the ErrNotFoundOrDirty err is returned. It is up
	// to the application code to handle the case in which a Dirty Write occurred or the document does not exist.
	DeleteConfig(ctx context.Context, name string, version uint64) error

	// More granular API

	// MarkForDeletion will mark the infrastructure in the document with the requested Config.Name and Config.Version for
	// deletion. If No documents with the given combination were marked for deletion the ErrNotFoundOrDirty err is returned. It is up
	// to the application code to handle the case in which a Dirty Write occurred or the document does not exist.
	MarkForDeletion(ctx context.Context, name string, version uint64) error
}

type Config struct {
	Version  uint64                   `bson:"version"`
	Name     string                   `bson:"name"`
	K8SCtx   KubernetesContext        `bson:"kubernetesContext"`
	Manifest Manifest                 `bson:"manifest"`
	Clusters map[string]*ClusterState `bson:"clusters"`
}

type KubernetesContext struct {
	Name      string `bson:"name"`
	Namespace string `bson:"namespace"`
}

type Manifest struct {
	Raw                 string `bson:"raw"`
	Checksum            []byte `bson:"checksum"`
	LastAppliedChecksum []byte `bson:"lastAppliedChecksum"`
	State               string `bson:"state"`
}

type ClusterState struct {
	Current  Clusters   `bson:"current"`
	InFlight *TaskEvent `bson:"inFlight"`
	State    Workflow   `bson:"state"`
}

type Clusters struct {
	K8s           []byte `bson:"k8s"`
	LoadBalancers []byte `bson:"loadBalancers"`
}

type StageKind string

const (
	Unknown     StageKind = ""
	Terraformer StageKind = "TERRAFORMER"
	Ansibler    StageKind = "ANSIBLER"
	KubeEleven  StageKind = "KUBE_ELEVEN"
	Kuber       StageKind = "KUBER"
)

type StageDescription struct {
	About      string `bson:"about"`
	ErrorLevel string `bson:"errorLevel"`
}

type SubPass struct {
	Kind        string           `bson:"kind"`
	Description StageDescription `bson:"description"`
}

type Stage struct {
	Kind        StageKind        `bson:"kind"`
	Description StageDescription `bson:"description"`
	SubPasses   []SubPass        `bson:"subPasses"`
}

type TaskEvent struct {
	Id           string   `bson:"id"`
	Timestamp    string   `bson:"timestamp"`
	Type         string   `bson:"event"`
	State        Clusters `bson:"state"`
	Task         []byte   `bson:"task"`
	Description  string   `bson:"description"`
	OnError      []byte   `bson:"onError"`
	Pipeline     []Stage  `bson:"pipeline"`
	CurrentStage uint32   `bson:"currentStage"`
}

type Workflow struct {
	Status      string `bson:"status"`
	Description string `bson:"description"`
	Timestamp   string `bson:"timestamp"`
}
