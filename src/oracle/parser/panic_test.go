// Every internal panic carries its format template, checked against the source rather than
// remembered (§7.8).
//
// A panic message that has already been through fmt.Sprintf is a different string for every
// instance — `event 42 …` and `event 7 …` are one bug that looks like two — so the template is
// what a crash report groups on, and it is the one part that cannot be recovered later: the
// stack, the classification and the crash file are all the driver's to add at the recover, but
// the template has to be captured at the call site or not at all
// (`oracle/driver/driver-implementation.md` §1).
//
// The check walks this package's own syntax trees, so it sees `panic(` in code and never in a
// comment or a string. It is scoped to this package because that is where the convention starts;
// generalizing it to the other passes is a sweep, not a redesign.
package parser

import (
	goast "go/ast"
	goparse "go/parser"
	gotoken "go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"
)

// panicFloor guards the walk rather than the code: a reader that found nothing would make this
// vacuous, which is the fail-open the golden and corpus floors guard against.
const panicFloor = 15

func TestEveryPanicCarriesItsTemplate(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	fset := gotoken.NewFileSet()
	total, scaffold := 0, 0

	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue // a test's panics are its own business
		}
		file, err := goparse.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		goast.Inspect(file, func(n goast.Node) bool {
			call, ok := n.(*goast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*goast.Ident); !ok || id.Name != "panic" || len(call.Args) != 1 {
				return true
			}
			total++
			switch arg := call.Args[0].(type) {
			case *goast.CallExpr:
				if isBugf(arg.Fun) {
					return true
				}
			case *goast.BasicLit:
				// The scaffold's sentinel is not an invariant violation, and this count is the
				// transition: it reaches zero when the last body lands.
				if strings.HasSuffix(strings.Trim(arg.Value, `"`), " is unimplemented") {
					scaffold++
					return true
				}
			}
			t.Errorf("%s: panic(%s)\n\twrite it panic(diagnostic.Bugf(…)) — the template is what a "+
				"crash report groups on, and fmt.Sprintf destroys it",
				fset.Position(call.Pos()), types.ExprString(call.Args[0]))
			return true
		})
	}

	if total < panicFloor {
		t.Fatalf("found %d panics, want at least %d; the walk is not reaching the package",
			total, panicFloor)
	}
	t.Logf("%d panics, %d of them the scaffold's unimplemented sentinel", total, scaffold)
}

func isBugf(fun goast.Expr) bool {
	sel, ok := fun.(*goast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*goast.Ident)
	return ok && pkg.Name == "diagnostic" && sel.Sel.Name == "Bugf"
}
