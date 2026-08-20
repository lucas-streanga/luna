package diagnostic

import "fmt"

// Bug is a panic payload: an invariant violated, which compiler §3.1 files under `I`, "this is
// a compiler bug". It is not a Diagnostic and deliberately carries no Code or Span: a panic has
// no position, and which code it becomes is the driver's to decide when it recovers
// (`oracle/driver/driver-implementation.md` §1).
//
// **Template is why this type exists.** A message that has been through fmt.Sprintf is a
// different string per instance. `event 42 …` and `event 7 …` are one bug wearing two faces, so
// so a crash report has nothing stable to group on. Everything else a report wants can be added
// at the recover: the stack (debug.Stack in the deferred function still sees the panicking
// frames), the classification (a Go fault satisfies runtime.Error, ours does not), the file, the
// version. The template is the only part that must be captured where the panic is written, which
// is the whole argument for spelling it `panic(diagnostic.Bugf(…))` rather than
// `panic(fmt.Sprintf(…))`.
//
// The panic stays at the call site rather than moving inside a helper, because Go's
// terminating-statement rule is keyed on the literal `panic`: a function that always panics is
// not one, so folding it in would demand a dead `return` after every use.
type Bug struct {
	Template string // the format string as written, which is what groups instances
	Message  string // the rendered text, which is what a human reads
}

// Bugf renders a bug and keeps the template it was rendered from.
func Bugf(format string, args ...any) Bug {
	return Bug{Template: format, Message: fmt.Sprintf(format, args...)}
}

// Error makes a Bug the panic value Go already prints well, and lets a recover pass it on as an
// ordinary error where that is what a caller wants.
func (b Bug) Error() string { return b.Message }
