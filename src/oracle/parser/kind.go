package parser

import "luna/oracle/token"

// Kind names a node: a grammar.md §0 nonterminal, a token kind for a leaf, or Error.
//
// **One kind space, not two** (parser-implementation.md §5), because the tree is homogeneous
// and every generic walk over it — the golden renderer, the formatter, the LSP's folding —
// wants one tag. The low range mirrors token.Kind's values so that Kind(tk.Kind) is a
// conversion and not a translation; uint16, because the two ranges together leave a byte
// fifteen spare values.
//
// The constants are hand-written, and kind_test.go's pin against §0 is what makes that safe:
// 105 generated constants are a file nobody reads, 105 written ones a file somebody reviewed
// once. It is the trade token.Kind already made.
type Kind uint16

// Unset is the zero value and names nothing, mirroring token.Unset: an uninitialized Kind must
// not masquerade as a real one. Distinct from Error, which the parser deliberately produces.
const Unset = Kind(token.Unset)

// tokenValues is the width of token.Kind's range — lexer §0's 134 kinds plus Unset. Pinned by
// TestTokenRangeIsMirrored, because a token added to §0 widens it and a node kind must not be
// sitting where it lands.
const tokenValues = 135

// firstNode is where the node kinds begin: one past token.Kind's last value, no gap.
const firstNode Kind = tokenValues

// The node kinds, in §0's order. A pure alternation is not among them — it is pure dispatch and
// always collapses, so it never reaches a tree (§5). `Type` is the one kept anyway: eliding it
// leaves a bare IDENT in type position indistinguishable from an expression's, which is the
// distinction R256 exists to make.
//
// `Prelude` is here because it is **not** a pure alternation, though it was counted as one until
// building a tree from the goldens said otherwise: `Prelude ::= PreludeItem*` is one symbol on
// the right only because the desugar made the repetition a helper, and a three-import file
// yields three children under it. §5 has the correction.
const (
	// §0.1 file and declarations
	File Kind = firstNode + iota
	Prelude
	PreludeItem
	ImportSpec
	ImportNames
	ImportName
	ModulePath
	Declaration
	BindingDecl
	Initializer
	Binder
	TestDecl
	Attribute
	AttrArgs
	AttrArg

	// §0.2 statements
	Block
	Statement
	SimpleStmt
	DeferStmt
	Modifier
	IfStmt
	WhileStmt
	ForeachStmt
	ForeachBinder

	// §0.3 expressions — the tiers survive whenever they fire, and only then
	Assignment
	AssignTarget
	WordPrefix
	Conditional
	Coalesce
	Disjunction
	Conjunction
	Equality
	Comparison
	RangeExpr
	Additive
	Multiplicative
	PrefixExpr
	ApplyExpr
	PostfixExpr
	Postfix
	Subscript
	ArgList
	Arg

	// §0.4 primary expressions
	Primary
	TableLit
	TableEntries
	TableEntry
	VariantLit
	VariantName
	FnLit
	GenLit
	ParamList
	Param
	MatchExpr
	MatchArms
	MatchArm
	GuardArms
	GuardArm
	TryCatchExpr
	CatchClause
	CatchBinder

	// §0.5 declaration literals
	ProtoLit
	ProtoItem
	MemberDecl
	Grants
	EnumLit
	VariantDecls
	VariantDecl
	PayloadShape
	PayloadFields
	PayloadField
	ErrorLit
	ErrorField
	ConstraintLit
	CapabilityLit
	AttributeLit
	AttrParams
	AttrParam
	UseClause
	CapList
	ProtoInit
	InitList

	// §0.6 types
	Type
	TypeOnly
	FnType
	TypeList
	UnionType
	IntersectType
	PostfixType
	PrimaryType

	// §0.7 patterns
	AltPattern
	PrimaryPattern
	LiteralPattern
	RangePattern
	TablePattern
	TablePatEntries
	TablePatEntry
	VariantPattern
	DestructurePattern
	DestrEntries
	DestrEntry
	KeyLit

	// §0.8 literals
	StringLit
	RegexLit
	CommandLit
	Splice

	// Error is the only kind with no nonterminal behind it, and its width is the whole
	// classification: zero means something should be here, positive means these tokens should
	// not be (§6.2). One kind rather than eleven, because splitting it along §11.2 would put a
	// second, unpinned copy of that table here (§6.3).
	Error
)

// nodeNames is keyed rather than positional, so reordering the block above cannot shift a name
// onto the wrong kind — the hazard token.Kind's own table guards against. The spellings are
// §0's, and kind_test.go checks them against §0 itself.
var nodeNames = [...]string{
	File:               "File",
	Prelude:            "Prelude",
	PreludeItem:        "PreludeItem",
	ImportSpec:         "ImportSpec",
	ImportNames:        "ImportNames",
	ImportName:         "ImportName",
	ModulePath:         "ModulePath",
	Declaration:        "Declaration",
	BindingDecl:        "BindingDecl",
	Initializer:        "Initializer",
	Binder:             "Binder",
	TestDecl:           "TestDecl",
	Attribute:          "Attribute",
	AttrArgs:           "AttrArgs",
	AttrArg:            "AttrArg",
	Block:              "Block",
	Statement:          "Statement",
	SimpleStmt:         "SimpleStmt",
	DeferStmt:          "DeferStmt",
	Modifier:           "Modifier",
	IfStmt:             "IfStmt",
	WhileStmt:          "WhileStmt",
	ForeachStmt:        "ForeachStmt",
	ForeachBinder:      "ForeachBinder",
	Assignment:         "Assignment",
	AssignTarget:       "AssignTarget",
	WordPrefix:         "WordPrefix",
	Conditional:        "Conditional",
	Coalesce:           "Coalesce",
	Disjunction:        "Disjunction",
	Conjunction:        "Conjunction",
	Equality:           "Equality",
	Comparison:         "Comparison",
	RangeExpr:          "RangeExpr",
	Additive:           "Additive",
	Multiplicative:     "Multiplicative",
	PrefixExpr:         "PrefixExpr",
	ApplyExpr:          "ApplyExpr",
	PostfixExpr:        "PostfixExpr",
	Postfix:            "Postfix",
	Subscript:          "Subscript",
	ArgList:            "ArgList",
	Arg:                "Arg",
	Primary:            "Primary",
	TableLit:           "TableLit",
	TableEntries:       "TableEntries",
	TableEntry:         "TableEntry",
	VariantLit:         "VariantLit",
	VariantName:        "VariantName",
	FnLit:              "FnLit",
	GenLit:             "GenLit",
	ParamList:          "ParamList",
	Param:              "Param",
	MatchExpr:          "MatchExpr",
	MatchArms:          "MatchArms",
	MatchArm:           "MatchArm",
	GuardArms:          "GuardArms",
	GuardArm:           "GuardArm",
	TryCatchExpr:       "TryCatchExpr",
	CatchClause:        "CatchClause",
	CatchBinder:        "CatchBinder",
	ProtoLit:           "ProtoLit",
	ProtoItem:          "ProtoItem",
	MemberDecl:         "MemberDecl",
	Grants:             "Grants",
	EnumLit:            "EnumLit",
	VariantDecls:       "VariantDecls",
	VariantDecl:        "VariantDecl",
	PayloadShape:       "PayloadShape",
	PayloadFields:      "PayloadFields",
	PayloadField:       "PayloadField",
	ErrorLit:           "ErrorLit",
	ErrorField:         "ErrorField",
	ConstraintLit:      "ConstraintLit",
	CapabilityLit:      "CapabilityLit",
	AttributeLit:       "AttributeLit",
	AttrParams:         "AttrParams",
	AttrParam:          "AttrParam",
	UseClause:          "UseClause",
	CapList:            "CapList",
	ProtoInit:          "ProtoInit",
	InitList:           "InitList",
	Type:               "Type",
	TypeOnly:           "TypeOnly",
	FnType:             "FnType",
	TypeList:           "TypeList",
	UnionType:          "UnionType",
	IntersectType:      "IntersectType",
	PostfixType:        "PostfixType",
	PrimaryType:        "PrimaryType",
	AltPattern:         "AltPattern",
	PrimaryPattern:     "PrimaryPattern",
	LiteralPattern:     "LiteralPattern",
	RangePattern:       "RangePattern",
	TablePattern:       "TablePattern",
	TablePatEntries:    "TablePatEntries",
	TablePatEntry:      "TablePatEntry",
	VariantPattern:     "VariantPattern",
	DestructurePattern: "DestructurePattern",
	DestrEntries:       "DestrEntries",
	DestrEntry:         "DestrEntry",
	KeyLit:             "KeyLit",
	StringLit:          "StringLit",
	RegexLit:           "RegexLit",
	CommandLit:         "CommandLit",
	Splice:             "Splice",
	Error:              "Error",
}

// String returns the spec's name for the kind rather than Go's: goldens and tree dumps are read
// against the spec, so they speak its vocabulary (token.Kind.String's own rule). A value above
// Error reports "UNKNOWN" rather than borrowing a token name, which would make a corrupt kind
// read as a plausible leaf.
func (k Kind) String() string {
	if k.IsToken() {
		return token.Kind(k).String()
	}
	if int(k) < len(nodeNames) {
		return nodeNames[k]
	}
	return "UNKNOWN"
}

// IsToken reports whether the kind came from the lexer — true of every leaf's, including the
// zero-width one a missing token leaves behind (§6.1).
func (k Kind) IsToken() bool { return k < firstNode }

// The three questions the builder asks of a kind before it will put one in a tree. They are
// unexported until something outside the package needs them — §8's view will want the first.

// isTrivia is what the golden renderer and the AST view both mean by "skip": whitespace, the
// shebang, and the two comment forms.
func isTrivia(k Kind) bool { return k.IsToken() && token.Kind(k).IsTrivia() }

// isNode reports whether the kind may be opened. The §0 nonterminals and Error, which §6.2 opens
// with positive width over tokens nobody could place; a value past Error names nothing.
func isNode(k Kind) bool { return !k.IsToken() && k <= Error }

// isSynthesisable reports whether the kind may be a zero-width leaf: the terminal an expect-site
// wanted (§6.1), or the Error marking an absent construct. Trivia is excluded because the parser
// never sees it and so can never expect it, and a zero-width trivia leaf would count as trivia in
// §2.3's placement invariant while belonging to no gap in the file. Unset names nothing.
func isSynthesisable(k Kind) bool {
	return k == Error || (k.IsToken() && k != Unset && !isTrivia(k))
}

// AllNodes returns every node kind, Error last. Derived from the range rather than from a
// second list, so a kind invented above appears here and has to answer to the pin.
func AllNodes() []Kind {
	ks := make([]Kind, 0, Error-firstNode+1)
	for k := firstNode; k <= Error; k++ {
		ks = append(ks, k)
	}
	return ks
}
