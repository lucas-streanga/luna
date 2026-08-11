// Package oracle is the root of Luna's complete Go implementation: a naive
// tree-walking interpreter running lex → parse → check → lower → eval.
//
// It has two duties and one constraint. It is the conformance oracle for
// differential testing (testing-strategy §3) and, until a stable Luna-written
// compiler exists, it is alpha v0 and the bootstrap interpreter that runs the
// production compiler's own source (R234, R241).
//
// The constraint: the production compiler shares no code with this tree
// (R241). It is written in Luna, so the language barrier enforces that — a
// Luna program cannot import a Go package. Nothing under compiler/ may be
// wired to oracle/value or oracle/std.
//
// Slow by design. Zero optimization, bignum-then-bound-check arithmetic: an
// oracle you optimize is an oracle you doubt (compiler §6.1).
//
// # Layout
//
// The packages divide in two, and the split is not cosmetic.
//
// Phases transform, and map to compiler §1: modules (§1.0 discovery, §1.2 import
// validation), lexer (§1.1), parser (§1.3), sema (§1.4), lower (§1.5), eval (the
// tree-walk that replaces §1.7's emission).
//
// Vocabulary is the data phases pass between them: token, source, diagnostic, ast, lir,
// value. These are not stages and are not nested inside the stage that happens to
// produce them, because each sits below several. source in particular is reached by
// everything that can report a location — parser for a syntax error, sema for a type
// error, eval for a runtime one, diagnostic because R240 makes every span (file, offset,
// length).
//
// An import path is a claim. Nesting source under lexer would make sema's type-error
// reporting import lexer/source, and the dependency graph would then assert that the
// type checker needs the lexer, which is false. Go's own tree splits the same way:
// go/token, go/scanner, go/ast, and go/parser are siblings, so ast and parser can use
// the vocabulary without the machinery.
package oracle
