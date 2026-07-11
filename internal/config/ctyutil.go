package config

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ctyBool returns the boolean value and true, or (false, false) when the
// value is null, unknown, or not a bool. Callers treat "not ok" as unset.
func ctyBool(v cty.Value) (bool, bool) {
	if v.IsNull() || !v.IsKnown() || v.Type() != cty.Bool {
		return false, false
	}
	return v.True(), true
}

func ctyString(v cty.Value) (string, bool) {
	if v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
		return "", false
	}
	return v.AsString(), true
}

func ctyInt(v cty.Value) (int, bool) {
	if v.IsNull() || !v.IsKnown() || v.Type() != cty.Number {
		return 0, false
	}
	f, _ := v.AsBigFloat().Float64()
	return int(f), true
}

// ctyStringSlice returns string elements of a list/tuple/set value.
// Non-string elements are skipped. Returns (nil, false) for non-collections.
func ctyStringSlice(v cty.Value) ([]string, bool) {
	if v.IsNull() || !v.IsKnown() || !v.CanIterateElements() {
		return nil, false
	}
	var out []string
	for it := v.ElementIterator(); it.Next(); {
		_, ev := it.Element()
		if s, ok := ctyString(ev); ok {
			out = append(out, s)
		}
	}
	return out, true
}

// ctyStringMap returns string-valued entries of an object/map value.
func ctyStringMap(v cty.Value) (map[string]string, bool) {
	if v.IsNull() || !v.IsKnown() || (!v.Type().IsObjectType() && !v.Type().IsMapType()) {
		return nil, false
	}
	out := make(map[string]string)
	for k, ev := range v.AsValueMap() {
		if s, ok := ctyString(ev); ok {
			out[k] = s
		}
	}
	return out, true
}

// ctyStringSliceMap returns entries whose values are string collections
// (used by block_order.nested_order).
func ctyStringSliceMap(v cty.Value) (map[string][]string, bool) {
	if v.IsNull() || !v.IsKnown() || (!v.Type().IsObjectType() && !v.Type().IsMapType()) {
		return nil, false
	}
	out := make(map[string][]string)
	for k, ev := range v.AsValueMap() {
		if ss, ok := ctyStringSlice(ev); ok {
			out[k] = ss
		}
	}
	return out, true
}

// attrVal evaluates a named attribute on body, returning (zero, false) when
// the attribute is absent or its expression fails to evaluate.
func attrVal(body *hclsyntax.Body, name string) (cty.Value, bool) {
	attr, ok := body.Attributes[name]
	if !ok {
		return cty.NilVal, false
	}
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return cty.NilVal, false
	}
	return val, true
}

// attrBool resolves a named boolean attribute, guarding against absence,
// eval errors, and wrong-typed values in one call.
func attrBool(body *hclsyntax.Body, name string) (bool, bool) {
	val, ok := attrVal(body, name)
	if !ok {
		return false, false
	}
	return ctyBool(val)
}

func attrString(body *hclsyntax.Body, name string) (string, bool) {
	val, ok := attrVal(body, name)
	if !ok {
		return "", false
	}
	return ctyString(val)
}

func attrInt(body *hclsyntax.Body, name string) (int, bool) {
	val, ok := attrVal(body, name)
	if !ok {
		return 0, false
	}
	return ctyInt(val)
}
