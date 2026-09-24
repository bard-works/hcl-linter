package config

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

// builtinPrefix marks an extends reference to a preset shipped in the binary,
// e.g. `extends = "builtin:terragrunt"`.
const builtinPrefix = "builtin:"

//go:embed presets/*.hcl
var presetFS embed.FS

// PresetNames lists the built-in presets available to `extends`.
func PresetNames() []string {
	entries, err := presetFS.ReadDir("presets")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".hcl"))
	}
	sort.Strings(names)
	return names
}

// presetSource returns the HCL source of a built-in preset.
func presetSource(name string) ([]byte, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".hcl")
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q (available: %s)",
			builtinPrefix+name, builtinPrefix+strings.Join(PresetNames(), ", "+builtinPrefix))
	}
	return data, nil
}
