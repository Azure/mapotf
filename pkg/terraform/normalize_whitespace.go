package terraform

import (
	"bytes"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// normalizeFileWhitespace strips stray blank lines that transforms like
// `sort_blocks_in_file` and `move_block` can leave behind when they remove a
// block from a file (hclwrite preserves the surrounding newline tokens) and
// re-append it elsewhere.
//
// Rules, all applied only at depth 0 (outside any block body) and outside
// heredoc content:
//
//   - leading blank lines are stripped (file starts on a non-empty line);
//   - runs of three or more consecutive newline tokens are collapsed to two
//     (i.e. at most one blank line between top-level constructs);
//   - the file always ends with exactly one trailing newline (zero trailing
//     blank lines), unless the file is empty.
//
// Whitespace inside a block (depth > 0) is preserved verbatim so the
// intentional blank lines emitted by `reorder_attributes` (between
// head/middle/tail and before nested blocks) are not affected. Heredoc
// content is preserved verbatim because the newlines inside a heredoc body
// are carried as part of the heredoc's TokenStringLit bytes rather than as
// standalone TokenNewline tokens.
//
// If `src` does not parse as valid HCL, it is returned unchanged so we never
// corrupt files we cannot reason about.
func normalizeFileWhitespace(src []byte) []byte {
	wf, diag := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diag.HasErrors() || wf == nil {
		return src
	}
	tokens := wf.BuildTokens(nil)
	return renderNormalizedTokens(tokens)
}

// renderNormalizedTokens walks `tokens` and emits their bytes with depth-0
// whitespace normalized per the rules documented on normalizeFileWhitespace.
//
// The token stream is the post-format hclwrite representation, so:
//   - braces are TokenOBrace / TokenCBrace,
//   - newlines outside heredocs are TokenNewline,
//   - heredoc bodies are bracketed by TokenOHeredoc / TokenCHeredoc and any
//     newlines inside them live in TokenStringLit bytes (no standalone
//     TokenNewline appears between them).
func renderNormalizedTokens(tokens hclwrite.Tokens) []byte {
	var (
		out             bytes.Buffer
		depth           int
		inHeredoc       bool
		pendingNewlines int
		emittedAny      bool
	)

	flushPending := func(maxNewlines int) {
		n := pendingNewlines
		if maxNewlines >= 0 && n > maxNewlines {
			n = maxNewlines
		}
		for i := 0; i < n; i++ {
			out.WriteByte('\n')
		}
		pendingNewlines = 0
	}

	writeRaw := func(tok *hclwrite.Token) {
		if tok.SpacesBefore > 0 {
			out.WriteString(strings.Repeat(" ", tok.SpacesBefore))
		}
		out.Write(tok.Bytes)
	}

	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenEOF:
			pendingNewlines = 0
			continue
		case hclsyntax.TokenOHeredoc:
			if pendingNewlines > 0 {
				if !emittedAny {
					pendingNewlines = 0
				} else if depth == 0 {
					flushPending(2)
				} else {
					flushPending(-1)
				}
			}
			emittedAny = true
			inHeredoc = true
			writeRaw(tok)
		case hclsyntax.TokenCHeredoc:
			inHeredoc = false
			writeRaw(tok)
		case hclsyntax.TokenNewline:
			if inHeredoc {
				writeRaw(tok)
				continue
			}
			pendingNewlines++
		default:
			if inHeredoc {
				writeRaw(tok)
				continue
			}
			if pendingNewlines > 0 {
				if !emittedAny {
					pendingNewlines = 0
				} else if depth == 0 {
					flushPending(2)
				} else {
					flushPending(-1)
				}
			}
			emittedAny = true
			writeRaw(tok)
			switch tok.Type {
			case hclsyntax.TokenOBrace:
				depth++
			case hclsyntax.TokenCBrace:
				if depth > 0 {
					depth--
				}
			}
		}
	}

	if !emittedAny {
		return nil
	}
	bs := out.Bytes()
	for len(bs) > 0 && bs[len(bs)-1] == '\n' {
		bs = bs[:len(bs)-1]
	}
	bs = append(bs, '\n')
	return bs
}
