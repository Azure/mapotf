package terraform

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

var _ Object = &RootBlock{}
var _ Object = &NestedBlock{}

type Object interface {
	EvalContext() cty.Value
}

func ListOfObject[T Object](objs []T) cty.Value {
	var values []cty.Value
	allTypes := make(map[string]cty.Type)
	canBuildList := true
	for _, b := range objs {
		value := b.EvalContext()
		values = append(values, value)
		if !canBuildList {
			continue
		}
		attributeTypes := value.Type().AttributeTypes()
		for n, t := range attributeTypes {
			if _, ok := allTypes[n]; !ok {
				allTypes[n] = t
				continue
			}
			if !allTypes[n].Equals(t) {
				if allTypes[n].IsListType() && t.IsListType() {
					elementType := mergeObjectType(allTypes[n].ElementType(), t.ElementType())
					if elementType == cty.NilType {
						canBuildList = false
						break
					}
					allTypes[n] = cty.List(elementType)
					continue
				}
				mergedType := mergeObjectType(allTypes[n], t)
				if mergedType == cty.NilType {
					canBuildList = false
					break
				}
				allTypes[n] = mergedType
			}
		}
	}
	if !canBuildList {
		return cty.TupleVal(values)
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
	if len(convertedValues) == 0 {
		return cty.ListValEmpty(finalType)
	}
	return cty.ListVal(convertedValues)
}

func mergeObjectType(t1, t2 cty.Type) cty.Type {
	if t1.IsTupleType() || t2.IsTupleType() {
		unifiedType, _ := convert.Unify([]cty.Type{t1, t2})
		return unifiedType
	}
	if t1.IsPrimitiveType() && t2.IsPrimitiveType() {
		return t1
	}
	if t1.IsCollectionType() && t2.IsCollectionType() {
		return mergeObjectTypeInCollection(t1, t2)
	}
	if !t1.IsObjectType() || !t2.IsObjectType() {
		return cty.NilType
	}
	newAttriubtes := make(map[string]cty.Type)
	for n, t := range t1.AttributeTypes() {
		newAttriubtes[n] = t
	}
	for n, t := range t2.AttributeTypes() {
		if _, ok := newAttriubtes[n]; !ok {
			newAttriubtes[n] = t
			continue
		}
		mergedType := mergeObjectType(newAttriubtes[n], t)
		if mergedType == cty.NilType {
			return cty.NilType
		}
		newAttriubtes[n] = mergedType
	}
	var allFields []string
	for n := range newAttriubtes {
		allFields = append(allFields, n)
	}
	return cty.ObjectWithOptionalAttrs(newAttriubtes, allFields)
}

func mergeObjectTypeInCollection(t1, t2 cty.Type) cty.Type {
	if t1.ElementType().IsObjectType() && t2.ElementType().IsObjectType() {
		mergedElementType := mergeObjectType(t1.ElementType(), t2.ElementType())
		if mergedElementType == cty.NilType {
			return cty.NilType
		}
		if t1.IsListType() {
			return cty.List(mergedElementType)
		}
		if t1.IsMapType() {
			return cty.Map(mergedElementType)
		}
		if t1.IsSetType() {
			return cty.Set(mergedElementType)
		}
	}
	return t1
}
