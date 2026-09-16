package pkg

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func validateObjectMergePatch(block *hclwrite.Block) error {
	for name, attr := range block.Body().Attributes() {
		if _, err := mergeObjectAttribute(nil, attr.Expr().BuildTokens(nil)); err != nil {
			return fmt.Errorf("invalid object merge patch for attribute %q: %w", name, err)
		}
	}
	for _, nested := range block.Body().Blocks() {
		if err := validateObjectMergePatch(nested); err != nil {
			return fmt.Errorf("nested block %q: %w", nested.Type(), err)
		}
	}
	return nil
}

func mergeObjectAttribute(target, patch hclwrite.Tokens) (hclwrite.Tokens, error) {
	patchSource := patch.Bytes()
	patchExpr, diag := hclsyntax.ParseExpression(patchSource, "object-patch", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, fmt.Errorf("cannot parse patch expression: %s", diag.Error())
	}
	patchExpr = unparenthesizedExpression(patchExpr)
	if comprehension, ok := patchExpr.(*hclsyntax.ForExpr); ok && comprehension.KeyExpr != nil {
		return nil, fmt.Errorf("object patch must be a literal object, not a for expression")
	}
	patchObject, patchIsObject := patchExpr.(*hclsyntax.ObjectConsExpr)
	var patchItems map[string]hclsyntax.ObjectConsItem
	if patchIsObject {
		var err error
		patchItems, err = staticObjectItems(patchObject)
		if err != nil {
			return nil, fmt.Errorf("invalid patch object: %w", err)
		}
	}
	if target == nil {
		return patch, nil
	}
	targetSource := target.Bytes()
	targetExpr, diag := hclsyntax.ParseExpression(targetSource, "object-target", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, fmt.Errorf("cannot parse target expression: %s", diag.Error())
	}
	targetObject, targetIsObject := unparenthesizedExpression(targetExpr).(*hclsyntax.ObjectConsExpr)
	if !patchIsObject {
		if targetIsObject {
			return nil, fmt.Errorf("cannot replace a literal object with a non-object patch when merge_object_attributes is enabled")
		}
		return patch, nil
	}
	if !targetIsObject {
		return nil, fmt.Errorf("object merge target must be a literal object")
	}
	targetItems, err := staticObjectItems(targetObject)
	if err != nil {
		return nil, fmt.Errorf("invalid target object: %w", err)
	}

	merged := make([]byte, 0, len(targetSource)+len(patchSource))
	cursor := 0
	for _, item := range targetObject.Items {
		key, _ := staticObjectKey(item.KeyExpr)
		replacement, exists := patchItems[key]
		if !exists {
			continue
		}
		valueRange := item.ValueExpr.Range()
		merged = append(merged, targetSource[cursor:valueRange.Start.Byte]...)
		merged = append(merged, replacement.ValueExpr.Range().SliceBytes(patchSource)...)
		cursor = valueRange.End.Byte
	}

	var additions [][]byte
	for _, item := range patchObject.Items {
		key, _ := staticObjectKey(item.KeyExpr)
		if _, exists := targetItems[key]; !exists {
			additions = append(additions, patchSource[item.KeyExpr.Range().Start.Byte:item.ValueExpr.Range().End.Byte])
		}
	}
	if len(additions) > 0 {
		tailStart := targetObject.OpenRange.End.Byte
		if len(targetObject.Items) > 0 {
			tailStart = targetObject.Items[len(targetObject.Items)-1].ValueExpr.Range().End.Byte
		}
		closeStart := targetObject.Range().End.Byte - 1
		tail, err := appendObjectItems(targetSource, targetObject, tailStart, closeStart, additions)
		if err != nil {
			return nil, err
		}
		merged = append(merged, targetSource[cursor:tailStart]...)
		merged = append(merged, tail...)
		cursor = closeStart
	}
	merged = append(merged, targetSource[cursor:]...)
	if _, diag := hclsyntax.ParseExpression(merged, "merged-object", hcl.InitialPos); diag.HasErrors() {
		return nil, fmt.Errorf("cannot safely merge object expression: %s", diag.Error())
	}
	tokens, diag := hclsyntax.LexExpression(merged, "merged-object", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, fmt.Errorf("cannot tokenize merged object expression: %s", diag.Error())
	}
	return writerTokens(tokens[:len(tokens)-1]), nil
}

func unparenthesizedExpression(expr hclsyntax.Expression) hclsyntax.Expression {
	for {
		parentheses, ok := expr.(*hclsyntax.ParenthesesExpr)
		if !ok {
			return expr
		}
		expr = parentheses.Expression
	}
}

func staticObjectItems(object *hclsyntax.ObjectConsExpr) (map[string]hclsyntax.ObjectConsItem, error) {
	items := make(map[string]hclsyntax.ObjectConsItem, len(object.Items))
	for _, item := range object.Items {
		key, ok := staticObjectKey(item.KeyExpr)
		if !ok {
			return nil, fmt.Errorf("object key at %s must be a literal name or string", item.KeyExpr.Range())
		}
		if _, exists := items[key]; exists {
			return nil, fmt.Errorf("duplicate object key %q at %s", key, item.KeyExpr.Range())
		}
		items[key] = item
	}
	return items, nil
}

func staticObjectKey(expr hclsyntax.Expression) (string, bool) {
	key, ok := expr.(*hclsyntax.ObjectConsKeyExpr)
	if !ok {
		return "", false
	}
	if !key.ForceNonLiteral {
		if name := hcl.ExprAsKeyword(key.Wrapped); name != "" {
			return name, true
		}
	}
	template, ok := unparenthesizedExpression(key.Wrapped).(*hclsyntax.TemplateExpr)
	if !ok || !template.IsStringLiteral() {
		return "", false
	}
	literal, ok := template.Parts[0].(*hclsyntax.LiteralValueExpr)
	if !ok || literal.Val.Type() != cty.String || literal.Val.IsNull() || !literal.Val.IsKnown() {
		return "", false
	}
	return literal.Val.AsString(), true
}

func appendObjectItems(source []byte, object *hclsyntax.ObjectConsExpr, tailStart, closeStart int, additions [][]byte) ([]byte, error) {
	tail := source[tailStart:closeStart]
	prefix := bytes.TrimRight(tail, " \t\r\n")
	suffix := tail[len(prefix):]
	multiline := bytes.Contains(object.Range().SliceBytes(source), []byte("\n"))
	for _, addition := range additions {
		multiline = multiline || bytes.Contains(addition, []byte("\n"))
	}

	var result []byte
	if multiline {
		result = append(result, prefix...)
		result = append(result, '\n')
		result = append(result, bytes.Join(additions, []byte("\n"))...)
		if !bytes.Contains(suffix, []byte("\n")) {
			result = append(result, '\n')
		}
		return append(result, suffix...), nil
	}

	tokens, diag := hclsyntax.LexExpression(tail, "object-tail", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, fmt.Errorf("cannot parse object separators: %s", diag.Error())
	}
	hasComma := false
	for _, token := range tokens {
		hasComma = hasComma || token.Type == hclsyntax.TokenComma
	}
	if len(object.Items) > 0 && !hasComma {
		result = append(result, ',')
	}
	result = append(result, prefix...)
	result = append(result, ' ')
	result = append(result, bytes.Join(additions, []byte(", "))...)
	return append(result, suffix...), nil
}
