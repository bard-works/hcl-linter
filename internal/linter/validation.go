package linter

import (
	"fmt"

	"github.com/bard-works/hcl-linter/internal/ast"
	"github.com/bard-works/hcl-linter/internal/config"
	"github.com/hashicorp/hcl/v2"
)

func checkNameValidationImpl(result *Result, blocks []ast.BlockInfo) {
	for _, block := range blocks {
		if len(block.Labels) > 0 {
			name := block.Labels[0]
			if !isValidName(name) {
				result.Issues = append(result.Issues, Issue{
					Severity: SeverityError,
					Rule:     "name_validation",
					Message:  fmt.Sprintf("invalid name %q: must contain only lowercase letters, numbers, and underscores, and must start with a letter", name),
					Location: block.Block.TypeRange,
				})
			}
		}

		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := getBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkNameValidationImpl(result, nestedBlocks)
		}
	}
}

func isValidName(name string) bool {
	if name == "" {
		return false
	}
	if !isLetter(rune(name[0])) {
		return false
	}
	for _, ch := range name {
		if !isLetter(ch) && !isDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func checkDuplicatesImpl(result *Result, blocks []ast.BlockInfo, _ *config.DuplicatesConfig) {
	seen := make(map[string]map[string]bool)

	for _, block := range blocks {
		blockType := block.Type

		if seen[blockType] == nil {
			seen[blockType] = make(map[string]bool)
		}

		var identifier string
		if len(block.Labels) > 0 {
			identifier = block.Labels[0]
		} else {
			identifier = ""
		}

		if seen[blockType][identifier] {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "duplicates",
				Message:  fmt.Sprintf("duplicate %s block with name/label %q", blockType, identifier),
				Location: block.Block.TypeRange,
			})
		} else {
			seen[blockType][identifier] = true
		}

		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := getBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkDuplicatesImpl(result, nestedBlocks, nil)
		}
	}
}

func checkRequiredFieldsImpl(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredFieldsConfig) {
	for _, block := range blocks {
		if block.Type == "include" {
			if cfg.Include != nil && cfg.Include.Expose {
				attrs := ast.GetBlockAttributes(block.Block.Body)
				if _, ok := attrs["expose"]; !ok {
					result.Issues = append(result.Issues, Issue{
						Severity:     SeverityError,
						Rule:         "required_fields",
						Message:      fmt.Sprintf("include block %q missing required field 'expose'", block.Labels),
						Location:     block.Block.TypeRange,
						SuggestedFix: "expose = true",
					})
				}
			}
		}

		if len(block.Block.Body.Blocks) > 0 {
			nestedBlocks := getBlockInfoFromBlocks(block.Block.Body.Blocks)
			checkRequiredFieldsImpl(result, nestedBlocks, cfg)
		}
	}
}

func checkRequiredBlocksImpl(result *Result, blocks []ast.BlockInfo, cfg *config.RequiredBlocksConfig) {
	blockCounts := make(map[string]int)
	for _, block := range blocks {
		blockCounts[block.Type]++
	}

	for _, req := range cfg.Required {
		count := blockCounts[req.Type]

		if req.Count == "once" && count != 1 {
			result.Issues = append(result.Issues, Issue{
				Severity: SeverityError,
				Rule:     "required_blocks",
				Message:  req.Error,
				Location: hcl.Range{
					Filename: result.File,
					Start:    hcl.Pos{Line: 1, Column: 1},
					End:      hcl.Pos{Line: 1, Column: 1},
				},
			})
		}
	}
}
