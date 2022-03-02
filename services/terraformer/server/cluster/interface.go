package cluster

type Clusters interface {
	Build() error
	Destroy() error
}
