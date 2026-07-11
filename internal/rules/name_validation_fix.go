package rules

import (
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
)

// renameKey identifies a block by its type and current label, so that
// blocks of different types sharing a label (dependency "web" vs include
// "web") rename independently.
type renameKey struct {
	blockType string
	oldLabel  string
}

func (r NameValidationRule) Fix(ctx *Context) (int, error) {
	newContent, count := FixNameValidation(ctx.Content, ctx.FilePath, ctx.Blocks, ctx.Config.NameValidation)
	if count == 0 {
		return 0, nil
	}
	ctx.Content = newContent
	return count, nil
}

// FixNameValidation renames block labels that fail cfg.Pattern (hyphens/spaces
// stripped to underscores) and rewrites only the token-level references to
// those labels (dot and index traversals). String values, comments, and
// prefix-sharing labels are structurally untouched because the rewrite
// operates on parsed hclwrite tokens rather than raw text.
func FixNameValidation(
	content []byte,
	filePath string,
	blocks []ast.BlockInfo,
	cfg *config.NameValidationConfig,
) ([]byte, int) {
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = defaultNamePattern
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return content, 0
	}

	blockSet := make(map[string]bool, len(cfg.Blocks))
	for _, b := range cfg.Blocks {
		blockSet[b] = true
	}

	renames := collectRenames(blocks, blockSet, regex)
	if len(renames) == 0 {
		return content, 0
	}

	file, diags := hclwrite.ParseConfig(content, filePath, hcl.InitialPos)
	if diags.HasErrors() {
		return content, 0
	}

	renamedLabels := renameBlockLabels(file.Body(), renames)
	if renamedLabels == 0 {
		return content, 0
	}
	rewriteReferences(file.Body(), renames)

	return file.Bytes(), renamedLabels
}

func collectRenames(blocks []ast.BlockInfo, blockSet map[string]bool, regex *regexp.Regexp) map[renameKey]string {
	renames := make(map[renameKey]string)
	collectRenamesRecursive(blocks, blockSet, regex, renames)
	return renames
}

func collectRenamesRecursive(
	blocks []ast.BlockInfo,
	blockSet map[string]bool,
	regex *regexp.Regexp,
	renames map[renameKey]string,
) {
	for _, block := range blocks {
		if len(block.Labels) > 0 && (len(blockSet) == 0 || blockSet[block.Type]) {
			label := block.Labels[0]
			if !regex.MatchString(label) {
				if newLabel := sanitizeLabel(label); newLabel != label {
					renames[renameKey{block.Type, label}] = newLabel
				}
			}
		}
		if len(block.Block.Body.Blocks) > 0 {
			collectRenamesRecursive(ast.GetBlockInfoFromBlocks(block.Block.Body.Blocks), blockSet, regex, renames)
		}
	}
}

func sanitizeLabel(label string) string {
	newLabel := strings.ReplaceAll(label, "-", "_")
	return strings.Join(strings.Fields(newLabel), "")
}

func renameBlockLabels(body *hclwrite.Body, renames map[renameKey]string) int {
	count := 0
	for _, block := range body.Blocks() {
		if len(block.Labels()) > 0 {
			key := renameKey{block.Type(), block.Labels()[0]}
			if newLabel, ok := renames[key]; ok {
				labels := block.Labels()
				labels[0] = newLabel
				block.SetLabels(labels)
				count++
			}
		}
		count += renameBlockLabels(block.Body(), renames)
	}
	return count
}

func rewriteReferences(body *hclwrite.Body, renames map[renameKey]string) {
	for _, attr := range body.Attributes() {
		rewriteReferenceTokens(attr.Expr().BuildTokens(nil), renames)
	}
	for _, block := range body.Blocks() {
		rewriteReferences(block.Body(), renames)
	}
}

// rewriteReferenceTokens mutates matched identifier/literal token bytes in
// place; BuildTokens returns pointers into the live hclwrite tree, so no
// SetAttributeRaw round-trip is needed.
func rewriteReferenceTokens(tokens hclwrite.Tokens, renames map[renameKey]string) {
	for i := range tokens {
		if newLabel, ok := matchDotRef(tokens, i, renames); ok {
			tokens[i+2].Bytes = []byte(newLabel)
			continue
		}
		if newLabel, ok := matchIndexRef(tokens, i, renames); ok {
			tokens[i+3].Bytes = []byte(newLabel)
		}
	}
}

// matchDotRef recognizes `type.label` traversals: TokenIdent TokenDot TokenIdent.
func matchDotRef(tokens hclwrite.Tokens, i int, renames map[renameKey]string) (string, bool) {
	if i+2 >= len(tokens) {
		return "", false
	}
	if tokens[i].Type != hclsyntax.TokenIdent ||
		tokens[i+1].Type != hclsyntax.TokenDot ||
		tokens[i+2].Type != hclsyntax.TokenIdent {
		return "", false
	}
	newLabel, ok := renames[renameKey{string(tokens[i].Bytes), string(tokens[i+2].Bytes)}]
	return newLabel, ok
}

// matchIndexRef recognizes `type["label"]` traversals: TokenIdent TokenOBrack
// TokenOQuote TokenQuotedLit TokenCQuote TokenCBrack.
func matchIndexRef(tokens hclwrite.Tokens, i int, renames map[renameKey]string) (string, bool) {
	if i+5 >= len(tokens) {
		return "", false
	}
	if tokens[i].Type != hclsyntax.TokenIdent ||
		tokens[i+1].Type != hclsyntax.TokenOBrack ||
		tokens[i+2].Type != hclsyntax.TokenOQuote ||
		tokens[i+3].Type != hclsyntax.TokenQuotedLit ||
		tokens[i+4].Type != hclsyntax.TokenCQuote ||
		tokens[i+5].Type != hclsyntax.TokenCBrack {
		return "", false
	}
	newLabel, ok := renames[renameKey{string(tokens[i].Bytes), string(tokens[i+3].Bytes)}]
	return newLabel, ok
}
