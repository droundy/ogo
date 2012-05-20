ogo
===

Ogo is an experimental go compiler.  It is written in go, and compiles
go to C, and invokes a C compiler to compile that.  It is not close to
complete, and may never be.  Its primary goal is not to be a good (or
complete) go compiler, but to be a platform for experimentation with
extensions to the go language.

Ogo is also intended to be the name of a temporary language, which
will have some degree of generics support.  This language has yet to
be defined.  It is intended to be "temporary" in the sense that I
expect that either its ideas will be incorporated into go itself, or
it will be abandoned eventually.  Until either of those happen, it
will need a separate name to distinguish ogo programs (with generics)
from go programs.

The design structure is to implement ogo primarily as a series of
go-to-go AST transformations, labeled (g2g) below.  Thus the C pretty
printer (and type checker, etc) will only need to handle a simpler
subset of the language.

Done
====

1. Parse go files (easy---use standard `go/parser`)

2. (g2g) Track go imports and combine into a single main file
(reasonably easy).  This simplifies the transformations considerably.
It's never going to be a fast way to compile things, but we've already
got a fast go compiler.

To Do
=====

In the following to-do list, some of the tasks require that others be
done first, but I haven't tabulated dependencies.  Many (particularly
of the g2g variety) can be implemented independently.

1. Finish C pretty printer using the ordinary go AST, with a subset
of the go syntax.

1. (g2g) Eliminate `if foo := bar(); foo` idiom.

1. (g2g) Eliminate `for a:=b; a<N; a++` idiom in favor of while-loop
for statements.

1. (g2g) Eliminate range statements over slices in for loops in favor
of explicit indexing and checking the length.

1. (g2g) Implement type checker, producing a map holding types of
every expression in the program.

1. (g2g) Add type casts to literals, e.g. transforming `0` into
`int(0)`, etc.

1. (g2g) Add type signatures to every `var` statement.

1. (g2g) Eliminate the `:=` operator in favor of `var` statements with
types.

1. (g2g) Eliminate `&` operator on local variables directly, changing
said local variables into pointer allocated with new.

1. Use Boehm garbage collector

1. Implement `type` as a builtin data type, and implement new in the
ogo language using this as its argument.  In C, `type` will be a
struct type, that will hold the size of the type, its name, and
eventually a bunch of pointers to its methods, equal to NULL if the
method hasn't been defined (so we can examine this to figure out
interfaces it satisfies).

1. Implement `make` in ogo

1. Implement `copy` in ogo

1. Implement `append` in ogo

1. Implement slices

1. Implement interfaces

1. (g2g) Transform interfaces than incorporate other interfaces into
simple flat interfaces (basically just copying over the methods).

1. (g2g) Make all type-casting explicit, i.e. insert a type cast when a
concrete type is passed to an `interface` argument of a function or
method.

Maybe To Do Some Day
====================

These are items that would be required to make `ogo` a valid Go1
compiler, that I don't have plans to implement (ever).  If they
interest you, however, you could work on them.

1. Implement `map`

2. Implement goroutines

3. Implement `chan`

4. Precise garbage collection

5. Implement `defer`

6. Implement `recover`

7. Implement `reflect`

8. Implement nested `struct` types
