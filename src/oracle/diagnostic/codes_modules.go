package diagnostic

// The module diagnostics: modules §12's table, one constant per row.
//
// All but one are raised by import validation (compiler §1.2): discovery answers *which
// files* and raises nothing (R250). MissingEntry is the exception, and it is not raised as a
// Diagnostic at all: it travels as the code on the error Discover returns, because at that
// point there is no file to anchor a span to. That is the shape source.Error established for
// ingress, and the reason it is still an M code is that the condition is about the root
// module, whoever ends up rendering it.
//
// Numbering is append-only and never reused (R240).
const (
	UnresolvedImport     Code = "M0001" // §3, §10, R251
	RootImport           Code = "M0002" // §3, R251
	ImportCycle          Code = "M0003" // §2, R251
	ImportOutsidePrelude Code = "M0004" // §4, R250
	MissingEntry         Code = "M0005" // §3
)

// modulesTitles is the title fixed to each module code, §12's Title column verbatim.
var modulesTitles = map[Code]string{
	UnresolvedImport:     "Unresolved import",
	RootImport:           "Root import",
	ImportCycle:          "Import cycle",
	ImportOutsidePrelude: "Import outside the prelude",
	MissingEntry:         "Missing entry",
}
