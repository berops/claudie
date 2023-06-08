package utils

import "github.com/berops/claudie/proto/pb"

func GetStaticNodepools(nps []*pb.NodePool) []*pb.NodePool {
	static := make([]*pb.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetStaticNodePool() != nil {
			static = append(static, n)
		}
	}
	return static
}

func GetDynamicNodepools(nps []*pb.NodePool) []*pb.NodePool {
	dynamic := make([]*pb.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetDynamicNodePool() != nil {
			dynamic = append(dynamic, n)
		}
	}
	return dynamic
}
