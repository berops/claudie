package templateUtils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasExtension(t *testing.T) {
	type V1 struct {
		Field1 string
		Field2 string
	}
	type V2 struct {
		Field3 string
		Field4 string
	}

	type Base struct {
		Base  string
		Base1 []string
		Base2 struct{ a, b, v string }
	}

	type ExtendedV1 struct {
		Base
		V1
	}

	type ExtendedV1_ struct {
		*Base
		Extension *V1
	}

	type ExtendedV2 struct {
		Base
		V1
		V2
	}

	assert.True(t, HasExtension(Base{}, "Base"))
	assert.False(t, HasExtension(Base{}, "V1"))

	l1 := &ExtendedV1{}
	l2 := &l1
	l3 := &l2
	l4 := &l3

	assert.False(t, HasExtension(ExtendedV1{}, "V2"))
	assert.True(t, HasExtension(ExtendedV1{}, "V1"))
	assert.True(t, HasExtension(ExtendedV1{}, "Base"))
	assert.True(t, HasExtension(ExtendedV1{}, "Base"))
	assert.True(t, HasExtension(l4, "Base"))
	assert.True(t, HasExtension(l4, "V1"))
	assert.True(t, HasExtension(l4, "Field1"))
	assert.False(t, HasExtension(l4, "V2"))

	l5 := &ExtendedV2{Base: l1.Base, V1: l1.V1}

	assert.True(t, HasExtension(ExtendedV2{}, "V2"))
	assert.True(t, HasExtension(ExtendedV2{}, "V1"))
	assert.True(t, HasExtension(ExtendedV2{}, "Base"))
	assert.True(t, HasExtension(ExtendedV2{}, "Base"))
	assert.True(t, HasExtension(l5, "Base"))
	assert.True(t, HasExtension(l5, "V1"))
	assert.True(t, HasExtension(l5, "Field1"))
	assert.True(t, HasExtension(l5, "Field3"))
	assert.True(t, HasExtension(l5, "Field4"))
	assert.True(t, HasExtension(l5, "V2"))
	assert.True(t, HasExtension(any(l5), "V2"))

	assert.False(t, HasExtension(any(new(int)), "Base"))
	assert.False(t, HasExtension(any(nil), "Base"))
	assert.False(t, HasExtension((*ExtendedV2)(nil), "V2"))
	assert.False(t, HasExtension(nil, "V2"))

	assert.False(t, HasExtension(new(ExtendedV1_), "Base"))
	assert.False(t, HasExtension(new(ExtendedV1_), "Extension"))
	assert.False(t, HasExtension(&ExtendedV1_{Base: new(Base), Extension: nil}, "Extension"))
	assert.True(t, HasExtension(&ExtendedV1_{Base: new(Base), Extension: nil}, "Base"))

	tmp := new(ExtendedV1_)
	assert.False(t, HasExtension(tmp, "Extension"))
	assert.False(t, HasExtension(tmp, "Base"))
	assert.False(t, HasExtension(tmp.Extension, "Field1"))
	tmp.Extension = new(V1)
	assert.True(t, HasExtension(tmp, "Extension"))
	assert.True(t, HasExtension(tmp.Extension, "Field1"))
	tmp.Base = new(Base)
	assert.True(t, HasExtension(tmp, "Base"))
	assert.True(t, HasExtension(tmp.Base, "Base"))
	assert.True(t, HasExtension(tmp.Base, "Base1"))
	assert.True(t, HasExtension(tmp.Base, "Base2"))
	assert.True(t, HasExtension(tmp.Base.Base2, "a"))
	assert.False(t, HasExtension(tmp.Base, "Base3"))
	tmp.Extension = nil
	assert.False(t, HasExtension(tmp, "Extension"))
}
