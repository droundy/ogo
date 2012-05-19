package main

// A handy program for compiling go code...

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
)

func parseFile(fset *token.FileSet, srcdir, f string) (parsedf *ast.File, err error) {
	parsedf, err = parser.ParseFile(fset, filepath.Join(srcdir, f), nil, parser.ParseComments)
	if err != nil {
		return
	}
	// fmt.Println("**********************")
	// fmt.Println(f)
	// fmt.Println("**********************")
	// fmt.Println(parsedf)
	// fmt.Println("**********************")
	// printer.Fprint(os.Stdout, fset, parsedf)
	// fmt.Println("**********************")
	//fmt.Println(parsedf.Unresolved)
	newunresolved := make([]*ast.Ident, 0, len(parsedf.Unresolved))
	for _, i := range parsedf.Unresolved {
		switch i.Name {
		case "string", "nil", "int", "bool", "byte", "uint", "error",
			"iota",
			"panic", "recover", "make", "len", "append", "new", "print",
			"false", "true":
		default:
			newunresolved = append(newunresolved, i)
		}
	}
	parsedf.Unresolved = newunresolved
	// fmt.Println(parsedf.Unresolved)
	return
}

func importPath(packages map[string](map[string]*ast.File), fset *token.FileSet, path, dir string) (fmap map[string]*ast.File, err error) {
	if _, ok := packages[path]; !ok {
		x, err := build.Import(path, dir, 0)
		if err != nil {
			return fmap, err
		}
		fmap = make(map[string]*ast.File)
		for _, f := range x.GoFiles {
			// fmt.Println("Looking up", f, "for import", path, "in directory", x.Dir)
			parsedf, err := parseFile(fset, x.Dir, f)
			if err != nil {
				fmt.Println("error on file", f, err)
			} else {
				fmap[f] = parsedf
			}
		}
		packages[path] = fmap
	}
	fmap = packages[path]

	for _, f := range packages[path] {
		for _, i := range f.Imports {
			subpath := i.Path.Value[1 : len(i.Path.Value)-1]
			packages[subpath], err = importPath(packages, fset, subpath, dir)
			if err != nil {
				panic(err)
			}
		}
	}
	return
}

func buildCommand(dir string) {
	fmt.Println("*****************************")
	fmt.Println(" building", dir)
	fmt.Println("*****************************")
	x, err := build.ImportDir(dir, 0)
	if err != nil {
		panic(err)
	}
	if !x.IsCommand() {
		fmt.Println("Use ogo on commands only!")
		os.Exit(1)
	}

	var fset token.FileSet
	packages := make(map[string](map[string]*ast.File))
	packages["main"] = make(map[string]*ast.File)
	for _, f := range x.GoFiles {
		parsedf, err := parseFile(&fset, dir, f)
		if err != nil {
			fmt.Println("error on file", f, err)
		} else {
			packages["main"][f] = parsedf
		}
	}
	_, err = importPath(packages, &fset, "main", dir)
	if err != nil {
		fmt.Println("Error importing stuff:", err)
	}
	mymain := TrackImports(packages)
	printer.Fprint(os.Stdout, &fset, mymain)
}

func main() {
	buildCommand(".")
	buildCommand("tests/hello")
}
