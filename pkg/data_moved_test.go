package pkg_test

import (
	"context"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestDataMoved_ExecuteDuringPlan(t *testing.T) {
	cases := []struct {
		desc          string
		tfCode        string
		expectedCount int
		expectedFrom  map[string]string
		expectedTo    map[string]string
	}{
		{
			desc: "single_moved_block_gets_index_zero",
			tfCode: `
moved {
  from = aws_instance.old
  to   = aws_instance.new
}`,
			expectedCount: 1,
			expectedFrom: map[string]string{
				"0": "aws_instance.old",
			},
			expectedTo: map[string]string{
				"0": "aws_instance.new",
			},
		},
		{
			desc: "multiple_moved_blocks_indexed_in_order",
			tfCode: `
moved {
  from = aws_instance.a
  to   = aws_instance.aa
}

moved {
  from = aws_instance.b
  to   = aws_instance.bb
}
`,
			expectedCount: 2,
			expectedFrom: map[string]string{
				"0": "aws_instance.a",
				"1": "aws_instance.b",
			},
			expectedTo: map[string]string{
				"0": "aws_instance.aa",
				"1": "aws_instance.bb",
			},
		},
		{
			desc:          "no_moved_blocks_yields_empty_result",
			tfCode:        `variable "x" { type = string }`,
			expectedCount: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": c.tfCode,
			}))
			defer stub.Reset()

			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, nil, nil, context.TODO())
			require.NoError(t, err)

			data := &pkg.DataMoved{
				BaseBlock: golden.NewBaseBlock(cfg, nil),
				BaseData:  &pkg.BaseData{},
			}
			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := data.Result
			values := result.AsValueMap()
			assert.Equal(t, c.expectedCount, len(values))
			for k, want := range c.expectedFrom {
				entry, ok := values[k]
				require.True(t, ok, "expected key %q in result", k)
				assert.Equal(t, cty.StringVal(want), entry.GetAttr("from"))
			}
			for k, want := range c.expectedTo {
				entry, ok := values[k]
				require.True(t, ok, "expected key %q in result", k)
				assert.Equal(t, cty.StringVal(want), entry.GetAttr("to"))
			}
		})
	}
}
