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
package oracle
