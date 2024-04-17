package pkg

import (
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"testing"
)

func TestJsonSerializeCtyValue(t *testing.T) {
	sut := cty.ObjectVal(map[string]cty.Value{
		"name": cty.StringVal("John"),
		"age":  cty.NumberIntVal(30),
	})
	marshal, err := ctyjson.Marshal(sut, cty.Object(map[string]cty.Type{
		"name": cty.String,
		"age":  cty.Number,
	}))
	require.NoError(t, err)
	println(string(marshal))
}
