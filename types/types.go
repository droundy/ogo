package types

import (
	"fmt"
	"go/ast"
	"go/token"
)

const (
	PointerSize int = 4
	IntSize     int = 4
)

func AlignSize(sz, al int) int {
	if sz%al != 0 {
		return (sz/al + 1) * al
	}
	return sz
}

type Type interface {
	Size() int
	Expr() ast.Expr
}

type TypeType struct {
}

func (t TypeType) Size() int {
	return PointerSize
}

type Int struct {
}

func (t Int) Size() int {
	return IntSize
}
func (t Int) Expr() ast.Expr {
	return ast.NewIdent("int")
}

type String struct {
}

func (t String) Size() int {
	return AlignSize(IntSize+PointerSize, PointerSize)
}
func (t String) Expr() ast.Expr {
	return ast.NewIdent("string")
}
func (t String) String() string {
	return "string"
}

type Function struct {
	Parameters, Results []Type
}

func (t Function) Size() int {
	// I plan to pass a bunch of arguments along with a pointer when
	// creating a closure.
	return AlignSize(IntSize+PointerSize+PointerSize, PointerSize)
}
func (t Function) String() string {
	return fmt.Sprint("func(", t.Parameters, ") ", t.Results)
}
func (t Function) Expr() ast.Expr {
	p := make([]*ast.Field, len(t.Parameters))
	r := make([]*ast.Field, len(t.Results))
	n := 0
	for i := range p {
		p[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fmt.Sprint("param", n))},
			Type:  t.Parameters[i].Expr()}
		n++
	}
	n = 0
	for i := range r {
		r[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fmt.Sprint("result", n))},
			Type:  t.Results[i].Expr()}
		n++
	}
	return &ast.FuncType{
		Params:  &ast.FieldList{List: p},
		Results: &ast.FieldList{List: r}}
}

type Method struct {
	Receiver            Type
	Parameters, Results []Type
}

func (t Method) Size() int {
	return PointerSize
}

// Here we go...

type Scope struct {
	types map[string]Type
	outer *Scope
}

func TypeCheck(bigfile *ast.File) map[string]Type {
	ts := make(map[string]Type)
	typeCheck(ts, bigfile.Decls)
	return ts
}

func typeCheck(t map[string]Type, ds []ast.Decl) {
	// First check types of all global variables and functions
	for _, d := range ds {
		typeDecl(t, d, ds)
	}
	// Finally, go into functions and check types inside
	//global := Scope{t, nil}
}

func typeDecl(t map[string]Type, d ast.Decl, ds []ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if _, ok := t[d.Name.Name]; ok {
			// No problem here, we've already done this!
		} else {
			if d.Recv == nil {
				args := []Type{}
				results := []Type{}
				for _, a := range d.Type.Params.List {
					for i := 0; i < len(a.Names); i++ {
						args = append(args, evalTypeExpr(a.Type, t, ds))
					}
				}
				if d.Type.Results != nil {
					for _, r := range d.Type.Results.List {
						for i := 0; i < len(r.Names); i++ {
							results = append(results, evalTypeExpr(r.Type, t, ds))
						}
					}
				}
				t[d.Name.Name] = Function{args, results}
				fmt.Println("Type of", d.Name.Name, "is", t[d.Name.Name])
			} else {
				panic("I don't yet handle methods!")
			}
		}
	case *ast.GenDecl:
		switch d.Tok {
		case token.IMPORT:
			// Nothing to do!
		case token.CONST:
			// TODO
		case token.TYPE:
			// TODO
		case token.VAR:
			for _, s := range d.Specs {
				s := s.(*ast.ValueSpec)
				var thist Type
				if s.Type != nil {
					thist = evalTypeExpr(s.Type, t, ds)
				} else {
					thist = findTypeOf(s.Values[0], t, ds)
					s.Type = thist.Expr()
				}
				for _, n := range s.Names {
					t[n.Name] = thist
				}
			}
		default:
			panic("Invalid token in GenDecl.Tok!")
		}
	default:
		panic(fmt.Sprintf("Unhandled case %T in typeDecl!", d))
	}
}

func evalTypeExpr(t ast.Expr, ts map[string]Type, ds []ast.Decl) Type {
	switch t.(type) {
	default:
		panic(fmt.Sprintf("Unknown type in evalTypeExpr: %T", t))
	}
	return Int{}
}

func findTypeOf(t ast.Expr, ts map[string]Type, ds []ast.Decl) Type {
	switch t := t.(type) {
	case *ast.BasicLit:
		switch t.Kind {
		case token.STRING:
			return String{}
		case token.INT:
			return Int{}
		default:
			panic(fmt.Sprint("BasicLit not understood yet: ", t.Kind, " which is ", t.Value))
		}
	default:
		panic(fmt.Sprintf("findTypeOf not implemented for %T", t))
	}
	panic("findTypeOf not implemented")
}
