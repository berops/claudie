package kubernetes

import (
	"github.com/Berops/platform/proto/pb"
)

type Kubernetes struct {
	DesiredK8s  *pb.K8Scluster
	CurrentK8s  *pb.K8Scluster
	ProjectName string
}

func (k Kubernetes) Build() error {

	return nil
}

func (k Kubernetes) Destroy() error {
	return nil
}
