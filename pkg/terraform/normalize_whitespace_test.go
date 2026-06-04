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
		{
			name: "default mode: adjacent unlabeled same-kind blocks (locals) get a blank line between them",
			in:   "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
		},
		{
			name: "default mode: three adjacent unlabeled same-kind blocks (locals) each get a blank line between them",
			in:   "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\nlocals {\n  c = 3\n}\n",
			want: "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n\nlocals {\n  c = 3\n}\n",
		},
		{
			name: "default mode: adjacent moved blocks get a blank line between them",
			in:   "moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
			want: "moved {\n  from = a.b\n  to   = c.d\n}\n\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
		},
		{
			name: "default mode: adjacent unlabeled different-kind blocks still get blank",
			in:   "locals {\n  a = 1\n}\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
			want: "locals {\n  a = 1\n}\n\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
		},
		{
			name: "default mode: labeled same-type blocks still get blank line (variables)",
			in:   "variable \"a\" {}\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "default mode: labeled same-type blocks still get blank line (resources)",
			in:   "resource \"x\" \"a\" {}\nresource \"x\" \"b\" {}\n",
			want: "resource \"x\" \"a\" {}\n\nresource \"x\" \"b\" {}\n",
		},
		{
			name: "default mode: existing single blank between same-kind unlabeled blocks is preserved",
			in:   "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
		},
		{
			name: "default mode: many blank lines between same-kind unlabeled blocks collapse to exactly one blank",
			in:   "locals {\n  a = 1\n}\n\n\n\n\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
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

// TestNormalizeFileWhitespace_KeepAdjacentSameKindUnlabeledBlocks exercises
// the opt-in behaviour enabled by
// NormalizeOptions.KeepAdjacentSameKindUnlabeledBlocks=true. With the flag
// on, adjacent root-level blocks that share the same leading identifier AND
// have no labels are kept adjacent (no blank line between them). Labeled
// sibling blocks (resource, data, variable, etc.) still get the blank line
// regardless, and unlabeled siblings of *different* kinds also still get
// the blank line.
func TestNormalizeFileWhitespace_KeepAdjacentSameKindUnlabeledBlocks(t *testing.T) {
	opts := NormalizeOptions{KeepAdjacentSameKindUnlabeledBlocks: true}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "opt-in: adjacent unlabeled same-kind blocks stay adjacent (locals)",
			in:   "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		},
		{
			name: "opt-in: three adjacent unlabeled same-kind blocks stay adjacent (locals)",
			in:   "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n\n\nlocals {\n  c = 3\n}\n",
			want: "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\nlocals {\n  c = 3\n}\n",
		},
		{
			name: "opt-in: adjacent unlabeled same-kind blocks stay adjacent (moved)",
			in:   "moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
			want: "moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
		},
		{
			name: "opt-in: adjacent unlabeled different-kind blocks still get blank",
			in:   "locals {\n  a = 1\n}\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
			want: "locals {\n  a = 1\n}\n\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
		},
		{
			name: "opt-in: labeled same-type blocks still get blank line (variables)",
			in:   "variable \"a\" {}\nvariable \"b\" {}\n",
			want: "variable \"a\" {}\n\nvariable \"b\" {}\n",
		},
		{
			name: "opt-in: labeled same-type blocks still get blank line (resources)",
			in:   "resource \"x\" \"a\" {}\nresource \"x\" \"b\" {}\n",
			want: "resource \"x\" \"a\" {}\n\nresource \"x\" \"b\" {}\n",
		},
		{
			name: "opt-in: comment between adjacent unlabeled same-kind blocks forces blank line",
			in:   "locals {\n  a = 1\n}\n# divider\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\n\n# divider\nlocals {\n  b = 2\n}\n",
		},
		{
			name: "opt-in: unlabeled then labeled does not group",
			in:   "locals {\n  a = 1\n}\nvariable \"x\" {}\n",
			want: "locals {\n  a = 1\n}\n\nvariable \"x\" {}\n",
		},
		{
			name: "opt-in: labeled then unlabeled does not group",
			in:   "variable \"x\" {}\nlocals {\n  a = 1\n}\n",
			want: "variable \"x\" {}\n\nlocals {\n  a = 1\n}\n",
		},
		{
			name: "opt-in: existing blank between same-kind unlabeled blocks is collapsed away",
			in:   "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		},
		{
			name: "opt-in: many blank lines between same-kind unlabeled blocks collapse to none",
			in:   "locals {\n  a = 1\n}\n\n\n\n\nlocals {\n  b = 2\n}\n",
			want: "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizeFileWhitespaceWithOptions([]byte(tc.in), opts))
			if got != tc.want {
				t.Errorf("normalizeFileWhitespaceWithOptions mismatch\n--- want ---\n%q\n--- got ---\n%q", tc.want, got)
			}
		})
	}
}

// TestNormalizeFileWhitespace_DefaultMatchesZeroOptions verifies that the
// shorthand normalizeFileWhitespace and the explicit
// normalizeFileWhitespaceWithOptions called with a zero NormalizeOptions
// value produce identical output, and that the package-level
// defaultNormalizeOptions starts at its zero value. This pins the
// "default-off" contract: SetNormalizeOptions has not been called in tests,
// so the production save path goes through the safe, always-blank rules.
func TestNormalizeFileWhitespace_DefaultMatchesZeroOptions(t *testing.T) {
	if got := NormalizeOptionsValue(); got != (NormalizeOptions{}) {
		t.Fatalf("expected zero-value default options, got %#v", got)
	}
	inputs := []string{
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		"moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
		"variable \"a\" {}\nvariable \"b\" {}\n",
		"",
		"\n\n",
	}
	for _, in := range inputs {
		viaDefault := string(normalizeFileWhitespace([]byte(in)))
		viaZeroOpts := string(normalizeFileWhitespaceWithOptions([]byte(in), NormalizeOptions{}))
		if viaDefault != viaZeroOpts {
			t.Errorf("default and zero-options differ for input %q:\ndefault: %q\nzero   : %q", in, viaDefault, viaZeroOpts)
		}
	}
}

// TestNormalizeFileWhitespace_SetNormalizeOptionsRoundtrip verifies the
// package-level setter is wired correctly: setting opt-in and re-running
// the shorthand entry point switches behaviour, and clearing it puts the
// behaviour back to the default.
func TestNormalizeFileWhitespace_SetNormalizeOptionsRoundtrip(t *testing.T) {
	saved := NormalizeOptionsValue()
	t.Cleanup(func() { SetNormalizeOptions(saved) })

	in := "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n"

	SetNormalizeOptions(NormalizeOptions{})
	if got := string(normalizeFileWhitespace([]byte(in))); got != "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n" {
		t.Errorf("after SetNormalizeOptions({}), got %q", got)
	}

	SetNormalizeOptions(NormalizeOptions{KeepAdjacentSameKindUnlabeledBlocks: true})
	if got := string(normalizeFileWhitespace([]byte(in))); got != "locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n" {
		t.Errorf("after SetNormalizeOptions(opt-in), got %q", got)
	}

	SetNormalizeOptions(NormalizeOptions{})
	if got := string(normalizeFileWhitespace([]byte(in))); got != "locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n" {
		t.Errorf("after reset to defaults, got %q", got)
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
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\nlocals {\n  c = 3\n}\n",
		"locals {\n  a = 1\n}\n\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
		"moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
		"locals {\n  a = 1\n}\n\n# divider\nlocals {\n  b = 2\n}\n",
	}
	for _, in := range inputs {
		first := normalizeFileWhitespace([]byte(in))
		second := normalizeFileWhitespace(first)
		if string(first) != string(second) {
			t.Errorf("not idempotent for input %q:\nfirst : %q\nsecond: %q", in, first, second)
		}
	}
}

// TestNormalizeFileWhitespace_Idempotent_KeepAdjacent verifies the opt-in
// rendering is also idempotent. The inputs include the canonical adjacent
// same-kind cases (locals, moved) that under opt-in mode collapse to no
// blank between them — and that collapsed form must round-trip cleanly.
func TestNormalizeFileWhitespace_Idempotent_KeepAdjacent(t *testing.T) {
	opts := NormalizeOptions{KeepAdjacentSameKindUnlabeledBlocks: true}
	inputs := []string{
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n",
		"locals {\n  a = 1\n}\n\nlocals {\n  b = 2\n}\n",
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\nlocals {\n  c = 3\n}\n",
		"moved {\n  from = a.b\n  to   = c.d\n}\nmoved {\n  from = e.f\n  to   = g.h\n}\n",
		"locals {\n  a = 1\n}\nlocals {\n  b = 2\n}\n\nterraform {\n  required_version = \"~> 1.0\"\n}\n",
		"locals {\n  a = 1\n}\n\n# divider\nlocals {\n  b = 2\n}\n",
		"variable \"a\" {}\nvariable \"b\" {}\n",
		"resource \"x\" \"a\" {}\nresource \"x\" \"b\" {}\n",
	}
	for _, in := range inputs {
		first := normalizeFileWhitespaceWithOptions([]byte(in), opts)
		second := normalizeFileWhitespaceWithOptions(first, opts)
		if string(first) != string(second) {
			t.Errorf("not idempotent (opt-in) for input %q:\nfirst : %q\nsecond: %q", in, first, second)
		}
	}
}
