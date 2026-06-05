package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &EphemeralData{}

type EphemeralData struct {
	*BaseData
	*golden.BaseBlock

	EphemeralType string    `hcl:"ephemeral_type,optional"`
	UseCount      bool      `hcl:"use_count,optional" default:"false"`
	UseForEach    bool      `hcl:"use_for_each,optional" default:"false"`
	Result        cty.Value `attribute:"result"`
}

func (ed *EphemeralData) Type() string {
	return "ephemeral"
}

func (ed *EphemeralData) ExecuteDuringPlan() error {
	src := ed.BaseBlock.Config().(*MetaProgrammingTFConfig).EphemeralBlocks()
	var matched []*terraform.RootBlock
	es := linq.From(src)
	if ed.EphemeralType != "" {
		es = es.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == ed.EphemeralType
		})
	}
	if ed.UseForEach {
		es = es.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).ForEach != nil
		})
	}
	if ed.UseCount {
		es = es.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Count != nil
		})
	}
	es.ToSlice(&matched)
	ephemeralBlocks := make(map[string]map[string]cty.Value)
	for _, b := range matched {
		ephemeralType := b.Labels[0]
		m, ok := ephemeralBlocks[ephemeralType]
		if !ok {
			m = make(map[string]cty.Value)
			ephemeralBlocks[ephemeralType] = m
		}
		m[b.Labels[1]] = b.EvalContext()
	}
	obj := make(map[string]cty.Value)
	for k, m := range ephemeralBlocks {
		obj[k] = cty.ObjectVal(m)
	}
	ed.Result = cty.ObjectVal(obj)
	return nil
}

func (ed *EphemeralData) String() string {
	d := cty.ObjectVal(map[string]cty.Value{
		"ephemeral_type": cty.StringVal(ed.EphemeralType),
		"use_count":      cty.BoolVal(ed.UseCount),
		"use_for_each":   cty.BoolVal(ed.UseForEach),
		"result":         ed.Result,
	})
	r, err := ctyjson.Marshal(d, d.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
