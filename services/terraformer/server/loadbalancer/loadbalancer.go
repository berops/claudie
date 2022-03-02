package loadbalancer

import "github.com/Berops/platform/proto/pb"

type Loadbalancer struct {
	LBcluster *pb.LBcluster
}

func (l Loadbalancer) Build() error {
	return nil
}

func (l Loadbalancer) Destroy() error {
	return nil
}
