package ast

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Parser struct {
	parser *hclparse.Parser
}

func NewParser() *Parser {
	return &Parser{
		parser: hclparse.NewParser(),
	}
}

func (p *Parser) ParseFile(path string) (*hcl.File, hcl.Diagnostics) {
	return p.parser.ParseHCLFile(path)
}

func (p *Parser) ParseContent(content []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	return p.parser.ParseHCL(content, filename)
}

func (p *Parser) ParseJSON(content []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	return p.parser.ParseJSON(content, filename)
}

type BlockInfo struct {
	Type      string
	Labels    []string
	Block     *hclsyntax.Block
	StartLine int
	EndLine   int
}

func GetTopLevelBlocks(file *hcl.File) []BlockInfo {
	var blocks []BlockInfo

	if syntaxBody, ok := file.Body.(*hclsyntax.Body); ok {
		for _, block := range syntaxBody.Blocks {
			startLine := block.TypeRange.Start.Line - 1
			endLine := block.Body.Range().End.Line - 1
			blocks = append(blocks, BlockInfo{
				Type:      block.Type,
				Labels:    block.Labels,
				Block:     block,
				StartLine: startLine,
				EndLine:   endLine,
			})
		}
	}

	return blocks
}

func GetBlockAttributes(body hcl.Body) map[string]hcl.Expression {
	attrs, _ := body.JustAttributes()
	result := make(map[string]hcl.Expression)
	for name, attr := range attrs {
		result[name] = attr.Expr
	}
	return result
}

func GetBlockBodyAttributes(body hcl.Body) (map[string]hcl.Expression, error) {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{},
		},
	}
	content, _, diags := body.PartialContent(schema)
	if diags.HasErrors() {
		return nil, diags
	}

	result := make(map[string]hcl.Expression)
	for name, attr := range content.Attributes {
		result[name] = attr.Expr
	}
	return result, nil
}

func GetBlockNestedBlocks(body hcl.Body, blockType string) []*hcl.Block {
	var blocks []*hcl.Block
	seen := make(map[string]bool)

	schemaWithLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: blockType, LabelNames: []string{"name"}},
		},
	}
	content1, _, _ := body.PartialContent(schemaWithLabel)
	for _, b := range content1.Blocks {
		key := b.TypeRange.String()
		if !seen[key] {
			seen[key] = true
			blocks = append(blocks, b)
		}
	}

	schemaNoLabel := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: blockType},
		},
	}
	content2, _, _ := body.PartialContent(schemaNoLabel)
	for _, b := range content2.Blocks {
		key := b.TypeRange.String()
		if !seen[key] {
			seen[key] = true
			blocks = append(blocks, b)
		}
	}

	return blocks
}

type AttributeInfo struct {
	Name      string
	Expr      hcl.Expression
	StartLine int
	EndLine   int
}

func GetTopLevelAttributes(file *hcl.File) []AttributeInfo {
	var attrs []AttributeInfo

	if syntaxBody, ok := file.Body.(*hclsyntax.Body); ok {
		for name, attr := range syntaxBody.Attributes {
			attrRange := attr.Expr.Range()
			startLine := attrRange.Start.Line - 1
			endLine := attrRange.End.Line - 1
			attrs = append(attrs, AttributeInfo{
				Name:      name,
				Expr:      attr.Expr,
				StartLine: startLine,
				EndLine:   endLine,
			})
		}
	}

	return attrs
}

func IsObjectAttribute(expr hcl.Expression) bool {
	_, ok := expr.(*hclsyntax.ObjectConsExpr)
	return ok
}

func GetAttributeRange(expr hcl.Expression) (startLine, endLine int) {
	if obj, ok := expr.(*hclsyntax.ObjectConsExpr); ok {
		r := obj.Range()
		startLine = r.Start.Line - 1
		endLine = r.End.Line - 1
		return
	}
	startLine = expr.Range().Start.Line - 1
	endLine = expr.Range().End.Line - 1
	return
}
