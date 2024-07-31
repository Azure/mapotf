package terraform

import (
	"fmt"
	"github.com/emirpasic/gods/stacks"
	"github.com/emirpasic/gods/stacks/arraystack"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/heimdalr/dag"
	"github.com/terraform-linters/tflint/terraform/addrs"
	"github.com/terraform-linters/tflint/terraform/lang"
	"strings"
)

var _ hclsyntax.Walker = &dependencyWalker{}

type dependencyWalker struct {
	b               *RootBlock
	d               *dag.DAG
	currentDynamics stacks.Stack
}

func newDependencyWalker(b *RootBlock, d *dag.DAG) *dependencyWalker {
	return &dependencyWalker{
		b:               b,
		d:               d,
		currentDynamics: arraystack.New(),
	}
}

func (w *dependencyWalker) Enter(node hclsyntax.Node) hcl.Diagnostics {
	if block, ok := node.(*hclsyntax.Block); ok && block.Type == "dynamic" {
		dynamicName := block.Labels[0]
		if iterator, ok := block.Body.Attributes["iterator"]; ok {
			iteratorValue, diag := iterator.Expr.Value(&hcl.EvalContext{})
			if diag.HasErrors() {
				return diag
			}
			dynamicName = iteratorValue.AsString()
		}
		w.currentDynamics.Push(dynamicName)
		return nil
	}

	attr, ok := node.(*hclsyntax.Attribute)
	if !ok {
		return nil
	}
	references, diag := w.references(attr.Expr)
	if diag.HasErrors() {
		return diag
	}

	for _, ref := range references {
		isDynamicRef := false
		for _, currentDynamic := range w.currentDynamics.Values() {
			if strings.HasPrefix(ref, fmt.Sprintf("resource.%s", currentDynamic.(string))) {
				isDynamicRef = true
			}
		}
		if isDynamicRef {
			continue
		}

		if w.b.dagAddress() == ref {
			continue
		}
		edgeExist, err := w.d.IsEdge(ref, w.b.dagAddress())
		if err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("error when check edge from %s to %s", ref, w.b.Address),
				Detail:   err.Error(),
			}}
		}
		if edgeExist {
			continue
		}
		if err := w.d.AddEdge(ref, w.b.dagAddress()); err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("error when add edge from %s to %s", w.b.Address, ref),
				Detail:   err.Error(),
			}}
		}
	}

	return nil
}

func (w *dependencyWalker) Exit(node hclsyntax.Node) hcl.Diagnostics {
	if block, ok := node.(*hclsyntax.Block); ok && block.Type == "dynamic" {
		_, _ = w.currentDynamics.Pop()
	}
	return nil
}

func (w *dependencyWalker) references(exp hcl.Expression) ([]string, hcl.Diagnostics) {
	refs, diag := lang.ReferencesInExpr(exp)
	if diag.HasErrors() {
		return nil, diag
	}
	var upstreams []string
	for _, ref := range refs {
		switch v := ref.Subject.(type) {
		case addrs.Resource:
			{
				if v.Mode == addrs.DataResourceMode {
					upstreams = append(upstreams, fmt.Sprintf("data.%s.%s", v.Type, v.Name))
					continue
				}
				upstreams = append(upstreams, fmt.Sprintf("resource.%s.%s", v.Type, v.Name))
			}
		case addrs.LocalValue:
			{
				upstreams = append(upstreams, fmt.Sprintf("local.%s", v.Name))
			}
		case addrs.ModuleCallInstanceOutput:
			{
				upstreams = append(upstreams, fmt.Sprintf("module.%s", v.Call.Call.Name))
			}
		case addrs.InputVariable:
			{
				upstreams = append(upstreams, fmt.Sprintf("var.%s", v.Name))
			}
		}
	}
	return upstreams, nil
}
