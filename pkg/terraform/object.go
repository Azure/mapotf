package terraform

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

var _ Object = &Block{}
var _ Object = &NestedBlock{}

type Object interface {
	EvalContext() cty.Value
}

func listOfObject[T Object](objs []T) cty.Value {
	var values []cty.Value
	allTypes := make(map[string]cty.Type)
	for _, b := range objs {
		value := b.EvalContext()
		values = append(values, value)
		attributeTypes := value.Type().AttributeTypes()
		for n, t := range attributeTypes {
			allTypes[n] = t
		}
	}
	var allFields []string
	linq.From(allTypes).Select(func(i interface{}) interface{} {
		return i.(linq.KeyValue).Key
	}).ToSlice(&allFields)
	finalType := cty.ObjectWithOptionalAttrs(allTypes, allFields)
	var convertedValues []cty.Value
	for _, v := range values {
		cv, err := convert.Convert(v, finalType)
		if err != nil {
			panic(err)
		}
		convertedValues = append(convertedValues, cv)
	}
	return cty.ListVal(convertedValues)
}
