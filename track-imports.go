package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func ManglePackageAndName(p, n string) string {
	out := strings.Replace(p+"_"+n, "/", "_", -1)
	out = strings.Replace(out, ".", "_", -1)
	out = strings.Replace(out, "-", "_", -1)
	return out
}

// Track imports simplifies all imports into a single large package
// with mangled names.  In the process, it drops functions that are
// never referred to.
func TrackImports(pkgs map[string](map[string]*ast.File)) (main *ast.File) {
	// Let's first set of the package we're going to generate...
	main = new(ast.File)
	main.Name = ast.NewIdent("main")
	initstmts := []ast.Stmt{} // this is where we'll stash the init statements...

	todo := make(map[string]struct{})
	todo["main.init"] = struct{}{}
	todo["main.main"] = struct{}{}
	done := make(map[string]struct{})

	for len(todo) > 0 {
		for pkgfn := range todo {
			pkg := strings.Split(pkgfn, ".")[0] // FIXME:  Need to split after last "." only
			fn := strings.Split(pkgfn, ".")[1]
			if _, ok := done[pkg+".init"]; !ok && fn != "init" {
				// We still need to init this package!
				todo[pkg+".init"] = struct{}{}
			}
			// fmt.Println("Working on package", pkg, "function", fn)
			// We need to look in all this package's files...
			for _, f := range pkgs[pkg] {
				// FIXME: it'd be marginally faster to first check if the
				// function we want is in this particular file.  On the other
				// hand, when there's only one file per package, that would be
				// slower...

				// First we'll track down the import declarations...
				sc := PackageScoping{
					Imports: make(map[string]string),
					Globals: make(map[string]string),
				}
				for _, d := range f.Decls {
					if i, ok := d.(*ast.GenDecl); ok && i.Tok == token.IMPORT {
						for _, s := range i.Specs {
							ispec := s.(*ast.ImportSpec) // This should always be okay!
							path, _ := strconv.Unquote(string(ispec.Path.Value))
							name := path
							if ispec.Name != nil {
								name = ispec.Name.Name
							} else {
								for _, f := range pkgs[path] {
									name = f.Name.Name
								}
							}
							sc.Imports[name] = path
						}
					} else if vdecl, ok := d.(*ast.GenDecl); ok && vdecl.Tok == token.VAR {
						for _, spec0 := range vdecl.Specs {
							spec := spec0.(*ast.ValueSpec)
							for _, n := range spec.Names {
								sc.Globals[n.Name] = pkg
							}
						}
					} else if tdecl, ok := d.(*ast.GenDecl); ok && vdecl.Tok == token.TYPE {
						for _, spec0 := range tdecl.Specs {
							spec := spec0.(*ast.TypeSpec)
							sc.Globals[spec.Name.Name] = pkg
						}
					} else if fdecl, ok := d.(*ast.FuncDecl); ok {
						sc.Globals[fdecl.Name.Name] = pkg
					}
				}
				// Now we'll go ahead and mangle things...
				for _, d := range f.Decls {
					if cdecl, ok := d.(*ast.GenDecl); ok && cdecl.Tok == token.CONST {
						fmt.Println("FIXME: I don't handle const yet at all... (ignoring)")
					} else if tdecl, ok := d.(*ast.GenDecl); ok && tdecl.Tok == token.TYPE {
						for _, spec0 := range tdecl.Specs {
							spec := spec0.(*ast.TypeSpec)
							if spec.Name.Name == fn {
								// fmt.Println("Got type declaration of", spec.Name)
								spec := *spec
								spec.Name = ast.NewIdent(fn)
								spec.Type = sc.MangleExpr(spec.Type)
								sc.MangleExpr(spec.Name)
								d := &ast.GenDecl{
									Tok:   token.TYPE,
									Specs: []ast.Spec{&spec},
								}
								main.Decls = append(main.Decls, d)
							}
						}
					} else if vdecl, ok := d.(*ast.GenDecl); ok && vdecl.Tok == token.VAR {
						for _, spec0 := range vdecl.Specs {
							spec := spec0.(*ast.ValueSpec)
							for i, n := range spec.Names {
								if n.Name == fn {
									// fmt.Println("I got variable", fn)
									nnew := *n
									sc.MangleExpr(&nnew)
									vs := []ast.Expr(nil)
									if len(spec.Values) > i {
										vs = append(vs, spec.Values[i])
										sc.MangleExpr(spec.Values[i])
									}
									sc.MangleExpr(spec.Type)
									d := ast.GenDecl{
										Tok: token.VAR,
										Specs: []ast.Spec{
											&ast.ValueSpec{
												Names:  []*ast.Ident{&nnew},
												Type:   spec.Type,
												Values: vs,
											},
										},
									}
									main.Decls = append(main.Decls, &d)
								}
							}
						}
					} else if fdecl, ok := d.(*ast.FuncDecl); ok {
						if fdecl.Name.Name == fn {
							// first, let's update the name... but in a copy of the
							// function declaration
							fdecl := *fdecl
							fdecl.Name = ast.NewIdent(pkg + "_" + fn)
							if fdecl.Type.Params != nil {
								for _, f := range fdecl.Type.Params.List {
									sc.MangleExpr(f.Type)
								}
							}
							if fdecl.Type.Results != nil {
								for _, f := range fdecl.Type.Results.List {
									sc.MangleExpr(f.Type)
								}
							}
							sc.MangleStatement(fdecl.Body)
							// fmt.Println("Dumping out", pkg, fn)
							main.Decls = append(main.Decls, &fdecl)
							if fn == "init" && fdecl.Recv == nil {
								initstmts = append(initstmts,
									&ast.ExprStmt{&ast.CallExpr{Fun: fdecl.Name}})
							}
						}
					}
				}
				// See what else we need to compile...
				for _, x := range sc.ToDo {
					if _, done := done[x]; !done {
						todo[x] = struct{}{}
					}
				}
			}
			delete(todo, pkgfn)
			done[pkgfn] = struct{}{}
		}
	}

	// Now we reverse the order, so that the declarations will be in
	// C-style order, not requiring forward declarations.
	newdecls := make([]ast.Decl, len(main.Decls))
	for i := range main.Decls {
		newdecls[i] = main.Decls[len(main.Decls)-i-1]
	}
	main.Decls = newdecls

	mainfn := new(ast.FuncDecl)
	mainfn.Name = ast.NewIdent("main")
	mainfn.Type = &ast.FuncType{Params: &ast.FieldList{}}
	initstmts = append(initstmts,
		&ast.ExprStmt{&ast.CallExpr{Fun: ast.NewIdent("main_main")}})
	mainfn.Body = &ast.BlockStmt{List: initstmts}
	main.Decls = append(main.Decls, mainfn)
	return main
}

type PackageScoping struct {
	Imports map[string]string
	Globals map[string]string
	ToDo    []string
}

func (sc *PackageScoping) Do(pkgid string) {
	sc.ToDo = append(sc.ToDo, pkgid)
}

func (sc *PackageScoping) MangleExpr(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.CallExpr:
		for i := range e.Args {
			e.Args[i] = sc.MangleExpr(e.Args[i])
		}
		sc.MangleExpr(e.Fun)
		if fn, ok := e.Fun.(*ast.Ident); ok {
			switch fn.Name {
			case "print", "println":
				sc.Do("runtime.print")
				sc.Do("runtime.print_int")
			case "new":
				sc.Do("runtime.malloc")
			}
		}
	case *ast.Ident:
		if pkg, ok := sc.Globals[e.Name]; ok {
			// It's a global identifier, so we need to mangle it...
			sc.Do(pkg + "." + e.Name)
			oldname := e.Name
			e.Name = ManglePackageAndName(pkg, e.Name)
			fmt.Println("Name is", e.Name, "from", pkg, oldname)
		} else {
			// Nothing to do here, it is a local identifier or builtin.
		}
	case *ast.BasicLit:
		// Nothing to do here, a literal is never imported.
	case *ast.BinaryExpr:
		e.X = sc.MangleExpr(e.X)
		e.Y = sc.MangleExpr(e.Y)
		// FIXME: We could do better here if we had type information...
		switch e.Op {
		case token.EQL:
			sc.Do("runtime.strings_equal")
		case token.NEQ:
			sc.Do("runtime.strings_unequal")
		}
	case *ast.UnaryExpr:
		sc.MangleExpr(e.X)
	case *ast.ParenExpr:
		sc.MangleExpr(e.X)
	case *ast.IndexExpr:
		e.X = sc.MangleExpr(e.X)
		e.Index = sc.MangleExpr(e.Index)
	case *ast.SliceExpr:
		e.X = sc.MangleExpr(e.X)
		e.Low = sc.MangleExpr(e.Low)
		e.High = sc.MangleExpr(e.High)
		sc.Do("runtime.slice_string")
	case *ast.StarExpr:
		sc.MangleExpr(e.X)
	case *ast.SelectorExpr:
		if b, ok := e.X.(*ast.Ident); ok {
			if theimp, ok := sc.Imports[b.Name]; ok {
				sc.Do(theimp + "." + e.Sel.Name)
				e.Sel.Name = ManglePackageAndName(theimp, e.Sel.Name)
				return e.Sel
			} else {
				fmt.Println("not a package: ", b.Name)
				fmt.Println("Imports are", sc.Imports)
				sc.MangleExpr(e.X)
			}
		} else {
			sc.MangleExpr(e.X)
		}
	case *ast.StructType:
		for _, f := range e.Fields.List {
			sc.MangleExpr(f.Type)
		}
	case *ast.CompositeLit:
		e.Type = sc.MangleExpr(e.Type)
		for i := range e.Elts {
			e.Elts[i] = sc.MangleExpr(e.Elts[i])
		}
	case *ast.MapType:
		e.Key = sc.MangleExpr(e.Key)
		e.Value = sc.MangleExpr(e.Value)
	case *ast.Ellipsis:
		e.Elt = sc.MangleExpr(e.Elt)
	case *ast.InterfaceType:
		for _, field := range e.Methods.List {
			field.Type = sc.MangleExpr(field.Type)
		}
	case *ast.ArrayType:
		e.Len = sc.MangleExpr(e.Len)
		e.Elt = sc.MangleExpr(e.Elt)
	case *ast.TypeAssertExpr:
		e.X = sc.MangleExpr(e.X)
		e.Type = sc.MangleExpr(e.Type)
	case *ast.FuncType:
		for _, field := range e.Params.List {
			field.Type = sc.MangleExpr(field.Type)
		}
		for _, field := range e.Results.List {
			field.Type = sc.MangleExpr(field.Type)
		}
	case *ast.FuncLit:
		for _, field := range e.Type.Params.List {
			field.Type = sc.MangleExpr(field.Type)
		}
		for _, field := range e.Type.Results.List {
			field.Type = sc.MangleExpr(field.Type)
		}
		sc.MangleStatement(e.Body)
	case *ast.KeyValueExpr:
		e.Key = sc.MangleExpr(e.Key)
		e.Value = sc.MangleExpr(e.Value)
	case nil:
		// Nothing to do with nil expression
	default:
		panic(fmt.Sprintf("Tracked weird expression of type %T", e))
	}
	return e
}

func (sc *PackageScoping) MangleStatement(st ast.Stmt) {
	switch st := st.(type) {
	case *ast.IncDecStmt:
		sc.MangleExpr(st.X)
	case *ast.BlockStmt:
		if st != nil && st.List != nil {
			for _, x := range st.List {
				sc.MangleStatement(x)
			}
		}
	case *ast.ForStmt:
		sc.MangleStatement(st.Post)
		sc.MangleExpr(st.Cond)
		sc.MangleStatement(st.Body)
		sc.MangleStatement(st.Init)
	case *ast.IfStmt:
		sc.MangleStatement(st.Init)
		sc.MangleExpr(st.Cond)
		sc.MangleStatement(st.Body)
		sc.MangleStatement(st.Else)
	case *ast.AssignStmt:
		for i := range st.Lhs {
			st.Lhs[i] = sc.MangleExpr(st.Lhs[i])
		}
		for i := range st.Rhs {
			st.Rhs[i] = sc.MangleExpr(st.Rhs[i])
		}
	case *ast.ExprStmt:
		sc.MangleExpr(st.X)
	case *ast.DeclStmt:
		switch decl := st.Decl.(type) {
		case *ast.GenDecl:
			if decl.Tok != token.VAR {
				panic(fmt.Sprint("I don't understand decl with tok", decl.Tok))
			}
			for _, spec := range decl.Specs {
				s := spec.(*ast.ValueSpec)
				sc.MangleExpr(s.Type)
				for _, v := range s.Values {
					sc.MangleExpr(v)
				}
			}
		default:
			panic("Weird Decl here...")
		}
	case *ast.ReturnStmt:
		for _, e := range st.Results {
			sc.MangleExpr(e)
		}
	case *ast.RangeStmt:
		st.Key = sc.MangleExpr(st.Key)
		st.Value = sc.MangleExpr(st.Value)
		st.X = sc.MangleExpr(st.X)
		sc.MangleStatement(st.Body)
	case *ast.SwitchStmt:
		st.Tag = sc.MangleExpr(st.Tag)
		sc.MangleStatement(st.Init)
		sc.MangleStatement(st.Body)
	case *ast.CaseClause:
		for i := range st.List {
			st.List[i] = sc.MangleExpr(st.List[i])
		}
		for _, st2 := range st.Body {
			sc.MangleStatement(st2)
		}
	case nil:
		// Nothing to do with a statement of type nil!
	default:
		panic(fmt.Sprintf("Tracked weird statement of type %T", st))
	}
}
