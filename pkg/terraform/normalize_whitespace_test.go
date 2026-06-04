package terraform

import (
	"testing"
)

func TestNormalizeFileWhitespace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "whitespace-only input",
			in:   "\n\n\n",
			want: "",
		},
		{
			name: "single block already canonical",
			in:   "variable \"x\" {\n  type = string\n}\n",
			want: "variable \"x\" {\n  type = string\n}\n",
		},
		{
			name: "strips leading blank lines",
			in:   "\n\n\n\nvariable \"x\" {\n  type = string\n}\n",
			want: "variable \"x\" {\n  type = string\n}\n",
		},
		{
			name: "strips trailing blank lines",
			in:   "variable \"x\" {\n  type = string\n}\n\n\n\n",
			want: "variable \"x\" {\n  type = string\n}\n",
		},
		{
			name: "collapses 3+ inter-block newlines to 2 (one blank line)",
			in:   "variable \"a\" {}\n\n\n\n\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "preserves single blank line between blocks",
			in:   "variable \"a\" {}\n\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "promotes adjacent blocks to one blank line between them",
			in:   "variable \"a\" {}\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "blank line is enforced between every pair of three adjacent blocks",
			in:   "variable \"a\" {}\nvariable \"b\" {}\nvariable \"c\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n\nvariable \"c\" {}\n",
		},
		{
			name: "blank line is enforced between mixed-kind adjacent root blocks",
			in:   "variable \"a\" {}\noutput \"b\" { value = 1 }\nresource \"r\" \"r\" {}\n",
			want: "variable \"a\" {}\n\noutput \"b\" { value = 1 }\n\nresource \"r\" \"r\" {}\n",
		},
		{
			name: "blank line enforced between root block and trailing comment",
			in:   "variable \"a\" {}\n# trailing\n",
			want: "variable \"a\" {}\n\n# trailing\n",
		},
		{
			name: "header comment stays attached to first block (no blank line)",
			in:   "# header\nvariable \"a\" {}\n",
			want: "# header\nvariable \"a\" {}\n",
		},
		{
			name: "leading comment for next block stays attached after blank line",
			in:   "variable \"a\" {}\n# header for b\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\n# header for b\nvariable \"b\" {}\n",
		},
		{
			name: "adjacent root comments are left alone",
			in:   "# c1\n# c2\n# c3\n",
			want: "# c1\n# c2\n# c3\n",
		},
		{
			name: "leaves blank lines inside block body untouched",
			in:   "variable \"x\" {\n  type    = string\n\n  default = \"\"\n\n  description = \"d\"\n}\n",
			want: "variable \"x\" {\n  type    = string\n\n  default = \"\"\n\n  description = \"d\"\n}\n",
		},
		{
			name: "no blank line inserted between adjacent nested blocks (depth > 0)",
			in:   "resource \"x\" \"y\" {\n  nested_one {\n    a = 1\n  }\n  nested_two {\n    b = 2\n  }\n}\n",
			want: "resource \"x\" \"y\" {\n  nested_one {\n    a = 1\n  }\n  nested_two {\n    b = 2\n  }\n}\n",
		},
		{
			name: "trailing block with no following content gets single trailing newline only",
			in:   "variable \"a\" {}\nvariable \"b\" {}\n\n\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "preserves heredoc body verbatim",
			in:   "locals {\n  doc = <<-EOT\n    line one\n\n\n\n    line two\n  EOT\n}\n",
			want: "locals {\n  doc = <<-EOT\n    line one\n\n\n\n    line two\n  EOT\n}\n",
		},
		{
			name: "blank line enforced between block containing a heredoc and next block",
			in:   "locals {\n  doc = <<-EOT\n    a\n  EOT\n}\nvariable \"x\" {}\n",
			want: "locals {\n  doc = <<-EOT\n    a\n  EOT\n}\n\nvariable \"x\" {}\n",
		},
		{
			name: "leading + mid + trailing combined",
			in:   "\n\n\nvariable \"a\" {}\n\n\n\nvariable \"b\" {}\n\n\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "invalid hcl returned unchanged",
			in:   "variable \"x\" {\n  type =.string\n}\n\n\n",
			want: "variable \"x\" {\n  type =.string\n}\n\n\n",
		},
		{
			name: "comment-only file",
			in:   "# just a comment\n",
			want: "# just a comment\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizeFileWhitespace([]byte(tc.in)))
			if got != tc.want {
				t.Errorf("normalizeFileWhitespace mismatch\n--- want ---\n%q\n--- got ---\n%q", tc.want, got)
			}
		})
	}
}

func TestNormalizeFileWhitespace_Idempotent(t *testing.T) {
	inputs := []string{
		"variable \"x\" {\n  type = string\n}\n",
		"\n\nvariable \"a\" {}\n\n\nvariable \"b\" {}\n\n",
		"variable \"a\" {}\nvariable \"b\" {}\nvariable \"c\" {}\n",
		"variable \"a\" {}\n# trailing\n",
		"# header\nvariable \"a\" {}\nvariable \"b\" {}\n",
		"locals {\n  s = <<-EOT\n    a\n\n    b\n  EOT\n}\n",
		"locals {\n  s = <<-EOT\n    a\n  EOT\n}\nvariable \"x\" {}\n",
		"resource \"x\" \"y\" {\n  a = 1\n\n  nested {\n    b = 2\n  }\n}\n",
		"resource \"x\" \"y\" {\n  nested_one {\n    a = 1\n  }\n  nested_two {\n    b = 2\n  }\n}\n",
	}
	for _, in := range inputs {
		first := normalizeFileWhitespace([]byte(in))
		second := normalizeFileWhitespace(first)
		if string(first) != string(second) {
			t.Errorf("not idempotent for input %q:\nfirst : %q\nsecond: %q", in, first, second)
		}
	}
}
