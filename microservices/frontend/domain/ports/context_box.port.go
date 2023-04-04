package ports

import "claudie/proto/generated"

type ContextBoxPort interface {
	GetConfigList( ) ([]*generated.Config, error)
	SaveConfig(config *generated.Config) error
	DeleteConfig(id string) error
}