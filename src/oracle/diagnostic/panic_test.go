// Every internal panic in the oracle carries its format template, checked against the source
// rather than remembered.
//
// A message that has been through fmt.Sprintf — or through string concatenation — is a different
// string for every instance, so `event 42 …` and `event 7 …` are one bug wearing two faces and a
// crash report has nothing stable to group on. Everything else a report wants can be added where
// the panic is caught: the stack, the classification, the file, the version. **The template is
// the only part that has to be captured where the panic is written**, which is the whole argument
// for `panic(diagnostic.Bugf(…))` (`oracle/driver/driver-implementation.md` §1.1).
//
// The check lives here because Bugf does, and the rule it pins is Bugf's: everything in the
// oracle that panics goes through it. It walks syntax trees rather than text, so it sees `panic(`
// in code and never in a comment or a string — which matters, since this package's own doc
// comments name the form.
package diagnostic_test

import (
	goast "go/ast"
	goparse "go/parser"
	gotoken "go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"luna/internal/spec"
)

// panicFloor guards the walk rather than the code: a reader that found nothing would make this
// vacuous, which is the same fail-open the golden and corpus floors guard against.
const panicFloor = 25

// The driver is exempt, and deliberately: it is the layer that **catches** a Bug and turns it
// into an `I` diagnostic, so its own panics answer to a design that is still being written
// (`driver-implementation.md` §1.2). It joins when that settles.
var panicExempt = map[string]bool{"driver": true}

func TestEveryPanicCarriesItsTemplate(t *testing.T) {
	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the module root: %v", err)
	}
	oracle := filepath.Join(root, "oracle")

	fset := gotoken.NewFileSet()
	total, scaffold := 0, 0
	perPackage := map[string]int{}

	err = filepath.WalkDir(oracle, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if panicExempt[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // a test's panics are its own business
		}
		file, perr := goparse.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		pkg := filepath.Base(filepath.Dir(path))
		goast.Inspect(file, func(n goast.Node) bool {
			call, ok := n.(*goast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*goast.Ident); !ok || id.Name != "panic" || len(call.Args) != 1 {
				return true
			}
			total++
			perPackage[pkg]++
			switch arg := call.Args[0].(type) {
			case *goast.CallExpr:
				if isBugf(arg.Fun) {
					return true
				}
			case *goast.BasicLit:
				// The parser scaffold's sentinel is not an invariant violation, and this count is
				// the transition: it reaches zero when the last body lands.
				if strings.HasSuffix(strings.Trim(arg.Value, `"`), " is unimplemented") {
					scaffold++
					return true
				}
			}
			t.Errorf("%s: panic(%s)\n\twrite it panic(diagnostic.Bugf(…)) — the template is what a "+
				"crash report groups on, and rendering at the call site destroys it",
				fset.Position(call.Pos()), types.ExprString(call.Args[0]))
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", oracle, err)
	}

	if total < panicFloor {
		t.Fatalf("found %d panics, want at least %d; the walk is not reaching the oracle",
			total, panicFloor)
	}
	t.Logf("%d panics over %d packages, %d of them the parser scaffold's sentinel",
		total, len(perPackage), scaffold)
}

// isBugf accepts both spellings: qualified from another package, bare from this one.
func isBugf(fun goast.Expr) bool {
	switch f := fun.(type) {
	case *goast.Ident:
		return f.Name == "Bugf"
	case *goast.SelectorExpr:
		pkg, ok := f.X.(*goast.Ident)
		return ok && pkg.Name == "diagnostic" && f.Sel.Name == "Bugf"
	}
	return false
}
