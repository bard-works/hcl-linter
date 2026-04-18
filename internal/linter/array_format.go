package linter

import (
	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

func checkArrayFormatImpl(result *Result, blocks []ast.BlockInfo) {
	for _, block := range blocks {
		if len(block.Block.Body.Attributes) > 0 {
			checkAttributeArrayFormat(result, block.Block.Body.Attributes, block.Block.TypeRange)
		}
		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := getBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkArrayFormatImpl(result, nestedBlocks)
		}
	}
}

func checkAttributeArrayFormat(result *Result, attrs map[string]*hclsyntax.Attribute, _ any) {
	for _, attr := range attrs {
		checkExpressionArrayFormat(result, attr.Expr, attr.Range())
	}
}

func checkExpressionArrayFormat(result *Result, expr hclsyntax.Expression, rangeObj any) {
	switch e := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		checkTupleConsFormat(result, e, rangeObj)
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			checkExpressionArrayFormat(result, item.ValueExpr, rangeObj)
		}
	}
}

func checkTupleConsFormat(result *Result, tuple *hclsyntax.TupleConsExpr, _ any) {
	allStrings := true
	for _, expr := range tuple.Exprs {
		if !isStringLiteral(expr) {
			allStrings = false
			break
		}
	}

	if allStrings && len(tuple.Exprs) > 1 {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Rule:     "array_format",
			Message:  "tuple of string literals should be written as a list for better readability",
		})
	}
}

func isStringLiteral(expr hclsyntax.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return e.Val.Type() == cty.String
	case *hclsyntax.TemplateExpr:
		return true
	default:
		return false
	}
}

func getBlockInfoFromBlocks(blocks []*hclsyntax.Block) []ast.BlockInfo {
	var result []ast.BlockInfo
	for _, block := range blocks {
		result = append(result, ast.BlockInfo{
			Type:   block.Type,
			Labels: block.Labels,
			Block:  block,
		})
	}
	return result
}
