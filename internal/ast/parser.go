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
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: blockType},
		},
	}
	content, _, _ := body.PartialContent(schema)
	return content.Blocks
}
