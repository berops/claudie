package ports

import "github.com/berops/claudie/proto/pb"

type ContextBoxPort interface {
	GetConfigList() ([]*pb.Config, error)
	SaveConfig(config *pb.Config) error
	DeleteConfig(id string) error
}
