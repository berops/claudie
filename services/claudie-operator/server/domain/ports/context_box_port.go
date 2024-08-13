package ports

import (
	"github.com/berops/claudie/proto/pb/spec"
)

type ContextBoxPort interface {
	GetAllConfigs() ([]*spec.Config, error)
	SaveConfig(config *spec.Config) error
	DeleteConfig(configName string) error
	PerformHealthCheck() error
}
