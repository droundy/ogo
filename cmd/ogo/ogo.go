package main

// A handy program for compiling go code...

import (
	"fmt"
	"github.com/droundy/ogo/cprinter"
	"github.com/droundy/ogo/transform"
	"github.com/droundy/ogo/types"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
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

func parseCommand(dir string) (*ast.File, *token.FileSet) {
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
	return transform.TrackImports(packages), &fset
}

func runGoBuildIn(dir string) (err error) {
	buildit := exec.Command("go", "build")
	buildit.Dir = dir
	return buildit.Run()
}

func buildC(f string) (err error) {
	return exec.Command("gcc", "-o", f[0:len(f)-2], f).Run()
}

func checkFor(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

func buildCommand(dir string) {
	fmt.Println("*****************************")
	fmt.Println(" building", dir)
	fmt.Println("*****************************")

	err := runGoBuildIn(dir)
	if err != nil {
		panic(fmt.Sprintln("Trouble building original file: ", dir, err))
	}

	// First parse the input file and concatenate all its necessary
	// imports.
	mymain, fset := parseCommand(dir)
	catdir := filepath.Join(dir, "concatenated")
	err = os.MkdirAll(catdir, 0777)
	if err != nil {
		panic(err)
	}
	goname := filepath.Join(catdir, filepath.Base(dir)+".go")
	f, err := os.Create(goname)
	if err != nil {
		panic(err)
	}
	printer.Fprint(f, fset, mymain)
	f.Close()
	err = runGoBuildIn(catdir)
	if err != nil {
		panic(fmt.Sprintln("Trouble building cat file: ", catdir, err))
	}

	// Now we typecheck the thing
	types.TypeCheck(mymain)

	// Finally, generate the C file
	cdir := filepath.Join(dir, "c")
	err = os.MkdirAll(cdir, 0777)
	if err != nil {
		panic(err)
	}
	cname := filepath.Join(cdir, filepath.Base(dir)+".c")
	f, err = os.Create(cname)
	fmt.Println("created", cname)
	if err != nil {
		panic(err)
	}
	cprinter.Fprint(f, fset, mymain)
	f.Close()
	err = buildC(cname)
	if err != nil && !checkFor(dir+"/CFAILS") {
		panic(err)
	}

	fmt.Println("Testing", dir, "...")
	outc, err := exec.Command(filepath.Join(catdir, filepath.Base(catdir))).CombinedOutput()
	if err != nil {
		panic("Error running concatenated file: " + err.Error())
	}
	outg, err := exec.Command(filepath.Join(dir, filepath.Base(dir))).CombinedOutput()
	if err != nil {
		panic("Error running raw file: " + err.Error())
	}
	if string(outc) != string(outg) {
		panic("outputs differ")
	}
	if !checkFor(dir + "/CFAILS") {
		outC, err := exec.Command(cname[0 : len(cname)-2]).CombinedOutput()
		if err != nil {
			fmt.Println("Error running C file: " + err.Error() + " command was " + cname[0:len(cname)-2])
		}
		if string(outC) != string(outg) {
			panic("C output differs:\n" + string(outC) + "\nversus:\n" + string(outg))
		}
	}
	fmt.Println("Tests pass!")
}

func main() {
	//buildCommand(".")
	tests, err := ioutil.ReadDir("tests")
	if err != nil {
		panic(err)
	}
	for _, test := range tests {
		if test.IsDir() {
			buildCommand("tests/" + test.Name())
		}
	}
}
