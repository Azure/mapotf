package terraform

import (
	"bytes"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// NormalizeOptions tunes normalizeFileWhitespace's inter-block layout
// behaviour. The zero value is the default behaviour applied by SaveToDisk
// when no CLI override is provided.
type NormalizeOptions struct {
	// KeepAdjacentSameKindUnlabeledBlocks, when true, suppresses the blank
	// line between adjacent root-level blocks that have no labels AND share
	// the same leading identifier (e.g. two `locals {}` or two `moved {}`).
	// Labeled blocks (`variable "a" {}` and `variable "b" {}`, two `resource`
	// blocks of the same type, etc.) are unaffected — they always get a
	// blank line between them.
	//
	// Default false: every pair of root blocks gets exactly one blank line
	// between them regardless of kind or label, matching the v0.1.1
	// normalizer's behaviour. This is the safe default because most existing
	// Terraform style guides and tooling pipelines expect a blank line
	// between every root block.
	//
	// Set to true via the `--keep-adjacent-blocks` CLI flag when a target
	// repository prefers grouping adjacent same-kind unlabeled blocks
	// together (matches the way `terraform fmt` leaves hand-written
	// adjacent `locals {}` blocks).
	KeepAdjacentSameKindUnlabeledBlocks bool
}

var defaultNormalizeOptions NormalizeOptions

// SetNormalizeOptions overrides the package-level options consulted by
// SaveToDisk when it calls normalizeFileWhitespace. Intended to be called
// once at startup from the CLI based on user flags. Not goroutine-safe;
// callers should set this before kicking off any transform that may write
// to disk.
func SetNormalizeOptions(opts NormalizeOptions) {
	defaultNormalizeOptions = opts
}

// NormalizeOptionsValue returns the current package-level NormalizeOptions
// previously set via SetNormalizeOptions (or the zero value if none was
// set). Exposed for tests and diagnostic logging.
func NormalizeOptionsValue() NormalizeOptions {
	return defaultNormalizeOptions
}

// normalizeFileWhitespace strips stray blank lines that transforms like
// `sort_blocks_in_file` and `move_block` can leave behind when they remove a
// block from a file (hclwrite preserves the surrounding newline tokens) and
// re-append it elsewhere. It also enforces a consistent inter-block layout so
// the same file always serialises the same way regardless of how transforms
// happened to mutate it.
//
// Rules, all applied only at depth 0 (outside any block body) and outside
// heredoc content:
//
//   - leading blank lines are stripped (the file starts on a non-empty line);
//   - after a `}` that closes a root-level block, the next non-newline token
//     in the file is preceded by exactly two newlines (i.e. exactly one
//     blank line between root-level constructs);
//   - elsewhere at depth 0, runs of three or more consecutive newlines are
//     collapsed to two (so an existing blank line is preserved, but multiple
//     are not), and shorter runs are left as-is;
//   - the file always ends with exactly one trailing newline (zero trailing
//     blank lines), unless the file is empty.
//
// Optional opt-in behaviour, enabled by setting
// NormalizeOptions.KeepAdjacentSameKindUnlabeledBlocks: when the
// just-closed root block has no labels (e.g. `locals {}`, `terraform {}`,
// `moved {}`, `removed {}`) and the next non-newline token at depth 0 is
// an identifier whose bytes match the closed block's leading identifier,
// the two blocks are emitted adjacent (one newline between them, no blank
// line). Same-kind unlabeled siblings — the canonical case being two
// `locals {}` blocks declared in the same file — group together the way
// they would have under `terraform fmt`. Labeled siblings (`variable "a"
// {}` and `variable "b" {}`, two `resource` blocks of the same type, etc.)
// still get the blank line — they represent distinct named entities and
// conventional style separates them.
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
	return normalizeFileWhitespaceWithOptions(src, defaultNormalizeOptions)
}

// normalizeFileWhitespaceWithOptions is the options-aware form of
// normalizeFileWhitespace. Tests use it directly; production code goes
// through normalizeFileWhitespace which reads the package-level
// defaultNormalizeOptions set by SetNormalizeOptions.
func normalizeFileWhitespaceWithOptions(src []byte, opts NormalizeOptions) []byte {
	wf, diag := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diag.HasErrors() || wf == nil {
		return src
	}
	tokens := wf.BuildTokens(nil)
	return renderNormalizedTokens(tokens, opts)
}

// renderNormalizedTokens walks `tokens` and emits their bytes with depth-0
// whitespace normalized per the rules documented on normalizeFileWhitespace.
//
// The token stream is the post-format hclwrite representation, so:
//   - braces are TokenOBrace / TokenCBrace,
//   - newlines outside heredocs are TokenNewline,
//   - block labels (e.g. the `"x"` and `"y"` in `resource "x" "y" {}`) are
//     bracketed by TokenOQuote / TokenCQuote with a TokenQuotedLit between,
//   - heredoc bodies are bracketed by TokenOHeredoc / TokenCHeredoc and any
//     newlines inside them live in TokenStringLit bytes (no standalone
//     TokenNewline appears between them).
func renderNormalizedTokens(tokens hclwrite.Tokens, opts NormalizeOptions) []byte {
	var (
		out                 bytes.Buffer
		depth               int
		inHeredoc           bool
		pendingNewlines     int
		emittedAny          bool
		lastWasRootCBrace   bool
		pendingBlockType    string
		pendingBlockLabeled bool
		currentBlockType    string
		currentBlockLabeled bool
		lastBlockType       string
		lastBlockLabeled    bool
	)

	emitNewlines := func(n int) {
		for i := 0; i < n; i++ {
			out.WriteByte('\n')
		}
		pendingNewlines = 0
	}

	flushPending := func(maxNewlines int) {
		n := pendingNewlines
		if maxNewlines >= 0 && n > maxNewlines {
			n = maxNewlines
		}
		emitNewlines(n)
	}

	writeRaw := func(tok *hclwrite.Token) {
		if tok.SpacesBefore > 0 {
			out.WriteString(strings.Repeat(" ", tok.SpacesBefore))
		}
		out.Write(tok.Bytes)
	}

	flushBeforeContent := func(tok *hclwrite.Token) {
		if pendingNewlines == 0 {
			return
		}
		if !emittedAny {
			pendingNewlines = 0
			return
		}
		if depth == 0 {
			if lastWasRootCBrace {
				sameKindUnlabeledAdjacent := opts.KeepAdjacentSameKindUnlabeledBlocks &&
					!lastBlockLabeled &&
					lastBlockType != "" &&
					tok != nil &&
					tok.Type == hclsyntax.TokenIdent &&
					string(tok.Bytes) == lastBlockType
				if sameKindUnlabeledAdjacent {
					emitNewlines(1)
				} else {
					emitNewlines(2)
				}
			} else {
				flushPending(2)
			}
			return
		}
		flushPending(-1)
	}

	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenEOF:
			pendingNewlines = 0
			continue
		case hclsyntax.TokenOHeredoc:
			flushBeforeContent(tok)
			emittedAny = true
			lastWasRootCBrace = false
			inHeredoc = true
			writeRaw(tok)
		case hclsyntax.TokenCHeredoc:
			inHeredoc = false
			writeRaw(tok)
			lastWasRootCBrace = false
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
			flushBeforeContent(tok)
			emittedAny = true
			writeRaw(tok)
			switch tok.Type {
			case hclsyntax.TokenOBrace:
				if depth == 0 {
					currentBlockType = pendingBlockType
					currentBlockLabeled = pendingBlockLabeled
					pendingBlockType = ""
					pendingBlockLabeled = false
				}
				depth++
				lastWasRootCBrace = false
			case hclsyntax.TokenCBrace:
				if depth > 0 {
					depth--
				}
				if depth == 0 {
					lastBlockType = currentBlockType
					lastBlockLabeled = currentBlockLabeled
					currentBlockType = ""
					currentBlockLabeled = false
					lastWasRootCBrace = true
				} else {
					lastWasRootCBrace = false
				}
			case hclsyntax.TokenIdent:
				if depth == 0 && pendingBlockType == "" {
					pendingBlockType = string(tok.Bytes)
				}
				lastWasRootCBrace = false
			case hclsyntax.TokenOQuote:
				if depth == 0 && pendingBlockType != "" {
					pendingBlockLabeled = true
				}
				lastWasRootCBrace = false
			default:
				lastWasRootCBrace = false
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
