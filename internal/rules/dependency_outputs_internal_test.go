package rules

import "testing"

// These cases were false negatives under the deleted regex-based parser
// (`^output\s+"(\w+)"`): it excluded dashes from \w, and only matched the
// header when it was the first token on its own line.
func TestDepParseOutputsFromTf_IrregularForms(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "dashed output name",
			content: `output "vpc-id" {}` + "\n",
			want:    "vpc-id",
		},
		{
			name:    "irregular spacing",
			content: `output   "id"   {}` + "\n",
			want:    "id",
		},
		{
			name:    "prefixed by same-line comment",
			content: `/* x */ output "id" {}` + "\n",
			want:    "id",
		},
		{
			name:    "no trailing newline",
			content: `output "id" {}`,
			want:    "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, ok := depParseOutputsFromTf([]byte(tt.content), "outputs.tf")
			if !ok {
				t.Fatalf("expected ok=true for valid HCL, got false")
			}
			if _, found := outputs[tt.want]; !found {
				t.Errorf("expected output %q to be found, got %v", tt.want, outputs)
			}
		})
	}
}

func TestDepParseOutputsFromTf_Unparseable(t *testing.T) {
	// Truncated block: the deleted regex still "found" this output on its
	// header line even though the file is not valid HCL.
	content := []byte(`output "a" {` + "\n")
	outputs, ok := depParseOutputsFromTf(content, "outputs.tf")
	if ok {
		t.Errorf("expected ok=false for truncated/invalid HCL, got outputs=%v", outputs)
	}
}

func TestDepGetOutputs_UnparseableFileMarksIncomplete(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir+"/good.tf", `output "a" { value = "x" }`+"\n")
	mustWriteFile(t, dir+"/bad.tf", `output "b" {`+"\n")

	outputs, _, incomplete := depGetOutputs(nil, dir)
	if !incomplete {
		t.Error("expected incomplete=true when one .tf file fails to parse")
	}
	if _, ok := outputs["a"]; !ok {
		t.Errorf("expected output from the well-formed file to still be collected, got %v", outputs)
	}
}
