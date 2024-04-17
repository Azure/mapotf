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

func ListOfObject[T Object](objs []T) cty.Value {
	var values []cty.Value
	allTypes := make(map[string]cty.Type)
	for _, b := range objs {
		value := b.EvalContext()
		values = append(values, value)
		attributeTypes := value.Type().AttributeTypes()
		for n, t := range attributeTypes {
			if _, ok := allTypes[n]; !ok {
				allTypes[n] = t
				continue
			}
			if !allTypes[n].Equals(t) {
				if allTypes[n].IsListType() && t.IsListType() {
					allTypes[n] = cty.List(mergeObjectType(allTypes[n].ElementType(), t.ElementType()))
					continue
				}
				allTypes[n] = mergeObjectType(allTypes[n], t)
			}
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

func mergeObjectType(t1, t2 cty.Type) cty.Type {
	newAttriubtes := make(map[string]cty.Type)
	for n, t := range t1.AttributeTypes() {
		newAttriubtes[n] = t
	}
	for n, t := range t2.AttributeTypes() {
		if _, ok := newAttriubtes[n]; !ok {
			newAttriubtes[n] = t
			continue
		}
		newAttriubtes[n] = mergeObjectType(newAttriubtes[n], t)
	}
	var allFields []string
	for n, _ := range newAttriubtes {
		allFields = append(allFields, n)
	}
	return cty.ObjectWithOptionalAttrs(newAttriubtes, allFields)
}