package usecases

type Usecases struct{}

type Cluster interface {
	Build() error
	Destroy() error
	Id() string
}
