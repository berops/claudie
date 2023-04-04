package manifest

type Manifest struct {
	Name string `validate:"required" yaml:"name"`
}