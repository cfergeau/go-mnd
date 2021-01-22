package checks

import (
	"go/ast"
	"go/token"
	"sync"

	"golang.org/x/tools/go/analysis"

	config "github.com/tommy-muehle/go-mnd/v2/config"
)

const ArgumentCheck = "argument"

// constantDefinitions is used to save lines (by number) which contain a constant definition.
var constantDefinitions = map[int]bool{}
var mu sync.RWMutex

type ArgumentAnalyzer struct {
	config *config.Config
	pass   *analysis.Pass
}

func NewArgumentAnalyzer(pass *analysis.Pass, config *config.Config) *ArgumentAnalyzer {
	return &ArgumentAnalyzer{
		pass:   pass,
		config: config,
	}
}

func (a *ArgumentAnalyzer) NodeFilter() []ast.Node {
	return []ast.Node{
		(*ast.GenDecl)(nil),
		(*ast.CallExpr)(nil),
	}
}

func (a *ArgumentAnalyzer) Check(n ast.Node) {
	switch expr := n.(type) {
	case *ast.CallExpr:
		a.checkCallExpr(expr)
	case *ast.GenDecl:
		if expr.Tok.String() == "const" {
			mu.Lock()
			constantDefinitions[a.pass.Fset.Position(expr.TokPos).Line] = true
			mu.Unlock()
		}
	}
}

func (a *ArgumentAnalyzer) checkCallExpr(expr *ast.CallExpr) {
	mu.RLock()
	ok := constantDefinitions[a.pass.Fset.Position(expr.Pos()).Line]
	mu.RUnlock()

	if ok {
		return
	}

	switch f := expr.Fun.(type) {
	case *ast.SelectorExpr:
		switch prefix := f.X.(type) {
		case *ast.Ident:
			if a.config.IsIgnoredFunction(prefix.Name + "." + f.Sel.Name) {
				return
			}
		}
	}

	for i, arg := range expr.Args {
		switch x := arg.(type) {
		case *ast.BasicLit:
			if !a.isMagicNumber(x) {
				continue
			}
			// If it's a magic number and has no previous element, report it
			if i == 0 {
				a.pass.Reportf(x.Pos(), reportMsg, x.Value, ArgumentCheck)
			} else {
				// Otherwise check the previous element type
				switch expr.Args[i-1].(type) {
				case *ast.ChanType:
					// When it's not a simple buffered channel, report it
					if a.isMagicNumber(x) {
						a.pass.Reportf(x.Pos(), reportMsg, x.Value, ArgumentCheck)
					}
				}
			}
		case *ast.BinaryExpr:
			a.checkBinaryExpr(x)
		}
	}
}

func (a *ArgumentAnalyzer) checkBinaryExpr(expr *ast.BinaryExpr) {
	switch x := expr.X.(type) {
	case *ast.BasicLit:
		if a.isMagicNumber(x) {
			a.pass.Reportf(x.Pos(), reportMsg, x.Value, ArgumentCheck)
		}
	}

	switch y := expr.Y.(type) {
	case *ast.BasicLit:
		if a.isMagicNumber(y) {
			a.pass.Reportf(y.Pos(), reportMsg, y.Value, ArgumentCheck)
		}
	}
}

func (a *ArgumentAnalyzer) isMagicNumber(l *ast.BasicLit) bool {
	return (l.Kind == token.FLOAT || l.Kind == token.INT) && !a.config.IsIgnoredNumber(l.Value)
}
