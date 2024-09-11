package pkg_test

import (
	"github.com/Azure/mapotf/pkg"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestToHclFunc(t *testing.T) {
	tests := []struct {
		name     string
		input    cty.Value
		expected string
	}{
		{
			name:     "String",
			input:    cty.StringVal("value"),
			expected: `"value"`,
		},
		{
			name:     "Number",
			input:    cty.NumberIntVal(42),
			expected: `42`,
		},
		{
			name:     "Bool",
			input:    cty.BoolVal(true),
			expected: `true`,
		},
		{
			name: "List",
			input: cty.ListVal([]cty.Value{
				cty.StringVal("one"),
				cty.StringVal("two"),
			}),
			expected: `["one", "two"]`,
		},
		{
			name: "Set",
			input: cty.SetVal([]cty.Value{
				cty.StringVal("one"),
				cty.StringVal("two"),
			}),
			expected: `["one", "two"]`,
		},
		{
			name: "Map",
			input: cty.MapVal(map[string]cty.Value{
				"key1": cty.StringVal("value1"),
				"key2": cty.StringVal("value2"),
			}),
			expected: `{
  key1 = "value1"
  key2 = "value2"
}`,
		},
		{
			name: "Object",
			input: cty.ObjectVal(map[string]cty.Value{
				"key": cty.StringVal("value"),
			}),
			expected: `{
  key = "value"
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pkg.ToHclFunc.Call([]cty.Value{tt.input})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if result.AsString() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result.AsString())
			}
		})
	}
}
