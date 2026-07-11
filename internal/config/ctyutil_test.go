package config

import (
	"reflect"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestCtyBool(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal bool
		wantOk  bool
	}{
		{"bool true", cty.True, true, true},
		{"bool false", cty.False, false, true},
		{"string", cty.StringVal("yes"), false, false},
		{"number", cty.NumberIntVal(1), false, false},
		{"list", cty.ListValEmpty(cty.String), false, false},
		{"map", cty.MapValEmpty(cty.String), false, false},
		{"null", cty.NullVal(cty.Bool), false, false},
		{"unknown", cty.UnknownVal(cty.Bool), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyBool(tc.v)
			if got != tc.wantVal || ok != tc.wantOk {
				t.Errorf(
					"ctyBool(%v) = (%v, %v), want (%v, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}

func TestCtyString(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal string
		wantOk  bool
	}{
		{"string", cty.StringVal("hello"), "hello", true},
		{"bool", cty.True, "", false},
		{"number", cty.NumberIntVal(1), "", false},
		{"list", cty.ListValEmpty(cty.String), "", false},
		{"map", cty.MapValEmpty(cty.String), "", false},
		{"null", cty.NullVal(cty.String), "", false},
		{"unknown", cty.UnknownVal(cty.String), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyString(tc.v)
			if got != tc.wantVal || ok != tc.wantOk {
				t.Errorf(
					"ctyString(%v) = (%q, %v), want (%q, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}

func TestCtyInt(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal int
		wantOk  bool
	}{
		{"number", cty.NumberIntVal(42), 42, true},
		{"bool", cty.True, 0, false},
		{"string", cty.StringVal("42"), 0, false},
		{"list", cty.ListValEmpty(cty.Number), 0, false},
		{"map", cty.MapValEmpty(cty.Number), 0, false},
		{"null", cty.NullVal(cty.Number), 0, false},
		{"unknown", cty.UnknownVal(cty.Number), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyInt(tc.v)
			if got != tc.wantVal || ok != tc.wantOk {
				t.Errorf(
					"ctyInt(%v) = (%d, %v), want (%d, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}

func TestCtyStringSlice(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal []string
		wantOk  bool
	}{
		{
			"list of strings",
			cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")}),
			[]string{"a", "b"},
			true,
		},
		{
			"tuple mixed skips non-string",
			cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.NumberIntVal(1)}),
			[]string{"a"},
			true,
		},
		{"set of strings", cty.SetVal([]cty.Value{cty.StringVal("a")}), []string{"a"}, true},
		{"bool", cty.True, nil, false},
		{"string", cty.StringVal("a"), nil, false},
		{"number", cty.NumberIntVal(1), nil, false},
		{"null", cty.NullVal(cty.List(cty.String)), nil, false},
		{"unknown", cty.UnknownVal(cty.List(cty.String)), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyStringSlice(tc.v)
			if !reflect.DeepEqual(got, tc.wantVal) || ok != tc.wantOk {
				t.Errorf(
					"ctyStringSlice(%v) = (%v, %v), want (%v, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}

func TestCtyStringMap(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal map[string]string
		wantOk  bool
	}{
		{
			"map of strings",
			cty.MapVal(map[string]cty.Value{"a": cty.StringVal("1")}),
			map[string]string{"a": "1"},
			true,
		},
		{
			"object mixed skips non-string",
			cty.ObjectVal(map[string]cty.Value{"a": cty.StringVal("1"), "b": cty.NumberIntVal(2)}),
			map[string]string{"a": "1"},
			true,
		},
		{"bool", cty.True, nil, false},
		{"string", cty.StringVal("a"), nil, false},
		{"list", cty.ListValEmpty(cty.String), nil, false},
		{"null", cty.NullVal(cty.Map(cty.String)), nil, false},
		{"unknown", cty.UnknownVal(cty.Map(cty.String)), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyStringMap(tc.v)
			if !reflect.DeepEqual(got, tc.wantVal) || ok != tc.wantOk {
				t.Errorf(
					"ctyStringMap(%v) = (%v, %v), want (%v, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}

func TestCtyStringSliceMap(t *testing.T) {
	cases := []struct {
		name    string
		v       cty.Value
		wantVal map[string][]string
		wantOk  bool
	}{
		{
			"object of lists",
			cty.ObjectVal(map[string]cty.Value{
				"terraform": cty.ListVal(
					[]cty.Value{cty.StringVal("before_hook"), cty.StringVal("after_hook")},
				),
			}),
			map[string][]string{"terraform": {"before_hook", "after_hook"}},
			true,
		},
		{"bool", cty.True, nil, false},
		{"string", cty.StringVal("a"), nil, false},
		{"number", cty.NumberIntVal(1), nil, false},
		{"list", cty.ListValEmpty(cty.String), nil, false},
		{"map", cty.MapValEmpty(cty.String), map[string][]string{}, true},
		{"null", cty.NullVal(cty.Map(cty.List(cty.String))), nil, false},
		{"unknown", cty.UnknownVal(cty.Map(cty.List(cty.String))), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ctyStringSliceMap(tc.v)
			if !reflect.DeepEqual(got, tc.wantVal) || ok != tc.wantOk {
				t.Errorf(
					"ctyStringSliceMap(%v) = (%v, %v), want (%v, %v)",
					tc.v,
					got,
					ok,
					tc.wantVal,
					tc.wantOk,
				)
			}
		})
	}
}
