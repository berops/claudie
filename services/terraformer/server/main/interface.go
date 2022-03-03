package main

type Cluster interface {
	Build() error
	Destroy() error

	GetName() string
}
