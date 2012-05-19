package main

// A handy program for compiling go code...

import (
	"fmt"
	"os"
	"go/build"
	"go/ast"
	"go/parser"
	"go/token"
	"go/printer"
)

func importFile(imports map[string]*ast.Object, path string) (pkg *ast.Object, err error) {
	if p, ok := imports[path]; ok {
		return p, nil
	}

	x, err := build.Import(path, ".", 0)
	if err != nil { return }

	var fset token.FileSet
	files := make(map[string]*ast.File)
	for _,f := range x.GoFiles {
		parsedf, err := parser.ParseFile(&fset, f, nil, parser.ParseComments)
		if err != nil {
			fmt.Println("error on file", f, err)
			return nil, err
		}
		files[f] = parsedf
		fmt.Println("**********************")
		fmt.Println(f)
		fmt.Println("**********************")
		printer.Fprint(os.Stdout, &fset, parsedf)
		fmt.Println("**********************")
	}

	scope := ast.NewScope(nil)
	p, err := ast.NewPackage(&fset, files, importFile, scope)
	pkg = ast.NewObj(ast.Pkg, p.Name)
	pkg.Data = scope
	return pkg, err
}

func main() {
	x, err := build.ImportDir(".", 0)
	if err != nil {
		panic(err)
	}
	fmt.Println(*x)
	fmt.Println("gofiles:", x.GoFiles)
	fmt.Println(x.IsCommand())

	var fset token.FileSet
	files := make(map[string]*ast.File)
	for _,f := range x.GoFiles {
		parsedf, err := parser.ParseFile(&fset, f, nil, parser.ParseComments)
		if err != nil {
			fmt.Println("error on file", f, err)
		} else {
			files[f] = parsedf
			fmt.Println("**********************")
			fmt.Println(f)
			fmt.Println("**********************")
			fmt.Println(parsedf)
			fmt.Println("**********************")
			printer.Fprint(os.Stdout, &fset, parsedf)
			fmt.Println("**********************")
			fmt.Println(parsedf.Unresolved)
			newunresolved := make([]*ast.Ident, 0, len(parsedf.Unresolved))
			for _,i := range parsedf.Unresolved {
				switch i.Name {
				case "string", "nil", "panic", "make":
				default:
					newunresolved = append(newunresolved, i)
				}
			}
			parsedf.Unresolved = newunresolved
			fmt.Println(parsedf.Unresolved)
		}
	}
	scope := ast.NewScope(nil)
	p, err := ast.NewPackage(&fset, files, nil, scope)
	if err != nil {
		fmt.Println("error on NewPackage:", err)
	}
	for i,_ := range p.Imports {
		fmt.Println("Imported: ", i)
		//fmt.Println(i, o.Data.(*ast.Scope).Objects)
	}
	for i,f := range p.Files {
		fmt.Println("Files: ", i)
		printer.Fprint(os.Stdout, &fset, f)
	}
}
