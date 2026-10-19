<div align="center">

# Jin

**A small, fast programming language — as easy to write as Python, with fewer footguns and much better performance.**

[![Go](https://img.shields.io/badge/Go-1.21-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-yellow.svg)](#license)
[![Build](https://github.com/jojin1709/jin/actions/workflows/go.yml/badge.svg)](https://github.com/jojin1709/jin/actions/workflows/go.yml)
[![Tests](https://img.shields.io/badge/tests-354%20passing-brightgreen)](#testing)
[![Stars](https://img.shields.io/github/stars/jojin1709/jin?style=social)](https://github.com/jojin1709/jin/stargazers)

A complete language implementation written from scratch — lexer, parser, type
checker, bytecode compiler, stack virtual machine, standard library and package
manager — in pure Go, with no third-party dependencies.

[Quick Start](#quick-start) · [Language Tour](#language-tour) · [Architecture](#architecture) · [Project Status](#project-status)

</div>

---

## Table of Contents

- [What is Jin?](#what-is-jin)
  - [Why Jin exists](#why-jin-exists)
  - [Why "Jin"?](#why-jin)
- [A first look](#a-first-look)
- [Quick Start](#quick-start)
  - [Prerequisites](#prerequisites)
  - [Build and run](#build-and-run)
  - [Command line reference](#command-line-reference)
- [Key capabilities](#key-capabilities)
- [The design rules](#the-design-rules)
- [Language tour](#language-tour)
  - [Built-ins](#built-ins)
- [Records and methods](#records-and-methods)
- [Maps](#maps)
- [Files](#files)
- [Error handling](#error-handling)
- [Types are optional, but checked](#types-are-optional-but-checked)
- [Closures](#closures)
- [Projects and imports](#projects-and-imports)
  - [Program arguments](#program-arguments)
  - [How a project is laid out](#how-a-project-is-laid-out)
- [Architecture](#architecture)
  - [The pipeline](#the-pipeline)
  - [Project layout](#project-layout)
  - [Kept in one place](#kept-in-one-place)
- [Performance](#performance)
- [Testing](#testing)
- [Project status](#project-status)
  - [What is in, and what is not](#what-is-in-and-what-is-not)
  - [Roadmap](#roadmap)
- [Developer](#developer)
- [License](#license)

---

## What is Jin?

Jin is a small, statically-checked, bytecode-compiled programming language. It is
designed around one idea: **the easy way to write something should also be the
correct way.**

Python is pleasant to write and unpleasant to debug, because most of its worst
surprises are quiet ones — a `1 + "1"` that fails at line 400 instead of at the
line you typed, a missing dictionary key that returns `None`, an `if` that
silently accepts a truthy integer, a typo that quietly creates a new global.
Jin keeps Python's readability and removes those surprises by making them
errors, and reports them *before* the program runs.

Underneath, Jin is not an interpreter wearing a hat. Source text is compiled to
bytecode and executed on a stack virtual machine, which is why it can hold its
own against CPython on call-heavy work.

<a id="why-jin-exists"></a>
<details>
<summary><strong>Why Jin exists</strong></summary>

Most "my own language" projects stop at a tree-walking interpreter. Jin was built
to go the whole distance, one stage at a time, and to prove each stage works
against the ones around it:

- **A real front end.** Lexer, Pratt parser and a flow-sensitive type checker.
- **A real back end.** A bytecode compiler and a stack VM with pooled constants,
  pre-allocated frame slots and shared call frames.
- **A real toolchain.** A module loader that reads a whole project, resolves
  imports, refuses cycles, and hands the same program to either engine.
- **A reference implementation.** The original tree-walking interpreter is kept
  as a second engine, and the test suite runs every program on *both* engines and
  demands byte-identical output. Two independent implementations agreeing is a
  far stronger correctness signal than either one passing on its own.

</details>

<a id="why-jin"></a>
<details>
<summary><strong>Why "Jin"?</strong></summary>

Short, easy to type, and not already taken by a language you would confuse it
with. It is also two letters away from "Jin" being the only thing the compiler
needs to know about you.

</details>

---

## A first look

```jin
fn fib(n) {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}

let fibs = []
for i in range(10) {
    fibs = push(fibs, fib(i))
}
print("fib: {join(fibs, \", \")}")
```

Output:

```
fib: 0, 1, 1, 2, 3, 5, 8, 13, 21, 34
```

Braces instead of whitespace, `{interpolation}` inside strings, optional type
annotations, and a standard library small enough to keep in your head.

---

## Quick Start

### Prerequisites

- **Go 1.21 or newer** — the only requirement. Jin has no third-party
  dependencies, so `go build` works offline on a fresh checkout.
- **Windows, Linux or macOS** — the CLI, the VM and the standard library are
  platform-independent; only the file built-ins touch the host.

### Build and run

```sh
go build -o jin.exe .

./jin.exe examples/tour.jin         # run a program (bytecode VM)
./jin.exe examples/soak.jin         # a bigger program: closures, recursion, loops
./jin.exe examples/collections.jin  # maps, lists and the built-ins for them
./jin.exe examples/types.jin        # the type-checked demo
./jin.exe examples/files.jin        # reading and writing files
./jin.exe examples/closures.jin     # shared captures and mutual recursion
./jin.exe run examples/greeting     # a project: two files, run by folder
./jin.exe run examples/greeting Jin # arguments reach the program as args()
./jin.exe                           # start the REPL
go test ./...
```

### Command line reference

| Command | What it does |
|---|---|
| `jin <file.jin> [args]` | Compile and run on the bytecode VM |
| `jin run [file\|folder] [args]` | The same thing, spelled out; a folder means a project |
| `jin interpret [file\|folder] [args]` | Run on the tree-walking reference interpreter |
| `jin check [file\|folder]` | Type-check without running anything |
| `jin build [folder]` | Check and compile a whole project, with a summary |
| `jin new <name>` | Start a project in `<name>/` |
| `jin compile <file.jin>` | Print the bytecode instead of running it |
| `jin parse <file.jin>` | Print the syntax tree |
| `jin tokens <file.jin>` | Print the raw token stream |
| `jin` | Run the project you are in, or start the REPL |
| `jin --version` / `jin --help` | Print the version, or the usage text |

Anything after the file name is handed to the program, where `args()` returns it.
A `--` before the arguments is allowed and ignored, so `jin run app.jin -- --help`
passes `--help` to the program instead of to Jin. A folder is a project when it
holds a `jin.toml`, and commands work from any folder inside a project, the way a
build tool does.

Each pipeline stage is available on its own, so you can watch the language being
translated one layer at a time. `run` and `interpret` must agree on every
program, and the test suite enforces exactly that — every program in the
differential corpus, every error message, every file in `examples/` and every
project under test has to produce byte-identical output on both back ends.

---

## Key capabilities

- **Errors caught before the program starts.** Undeclared names, wrong argument
  counts, mismatched types, non-`bool` conditions and bad indexing are all
  reported by `jin check`, with the line and column they came from.
- **A real compiler and VM.** Names resolve to slots before execution; small
  integers, booleans and `nil` are pooled; call frames are reused.
- **Genuine closures with shared captures.** A captured variable is the
  variable, not a snapshot, and each loop pass gets a fresh binding.
- **Structs and methods.** User-defined record types declared with `struct`,
  constructor calls, dotted field access and mutation, and methods declared
  with receiver syntax (`fn (p: Point)`).
- **Fast ordered maps.** Ordered entries, `int` / `str` / `bool` keys, and
  a missing key that is an error rather than a silent `nil`.
- **One obvious way to do things.** No decorators, no metaclasses, no dunder
  magic, no implicit truthiness.
- **Projects, with namespaced files.** A folder with a `jin.toml` is a project;
  `import "utils" as u` makes another file's top-level names reachable as
  `u.helper`, so two files on one project can never collide by accident.
- **Two engines, one meaning.** A bytecode VM and a tree-walking interpreter
  that the test suite forces to agree, byte for byte.

---

## The design rules

1. **Braces, not invisible whitespace.** No broken programs because of a stray tab.
2. **One obvious way to do things.** No decorators, no metaclasses, no dunder magic.
3. **No silent type mixing.** `1 + 1.5` is an error, not a surprise. Convert with `int()` or `float()`.
4. **Conditions must be `bool`.** `if 1 { }` is rejected instead of quietly being truthy.
5. **No accidental globals.** Assigning to a name that was never declared is an error.
6. **A missing key is an error.** `m["nope"]` is not quietly `nil`; ask with `has(m, "nope")`.
7. **A captured variable is shared, not copied.** Two functions written side by
   side see each other's writes to it, and each pass of a loop gets its own
   binding rather than the last value the loop reached.
8. **Fast by default.** Source compiles to bytecode and runs on a stack VM. The
   tree-walking interpreter is still there for cross-checking, but it is not
   what `run` uses.
9. **A file's names belong to that file.** Nothing in another file is in scope
   until you import it and name it through the alias, and another file's names
   are read-only from the outside.

---

## Language tour

```jin
# comments start with a hash

let x = 5                  # let declares
let name = "Jin"
x = x + 1                  # assign to an existing name
let a = 1; let b = 2       # a ';' separates statements, and is optional

fn add(a, b) { return a + b }        # implicit return also works: fn sq(x) { x * x }

if x > 5 { print("big") } else if x == 5 { print("five") } else { print("small") }

for n in [1, 2, 3] { print(n) }      # also: for ch in "abc", for i in range(10)
while x > 0 { x = x - 1; if x == 2 { break } }

let f = fn(v) { v * 2 }              # anonymous functions
print("{name} has {len(name)} letters")   # string interpolation
print("use {{braces}} for literal braces")

let person = {"name": "Ada", "age": 36}   # a map
print(person["name"], has(person, "job")) # Ada false
for field in person { print(field) }      # maps loop over their keys
```

### Built-ins

| Group | Functions |
|---|---|
| Output | `print` |
| Numbers | `abs`, `min`, `max`, `floor`, `ceil`, `round`, `sqrt`, `pow`, `random`, `int`, `float`, `str`, `type` |
| Strings | `len`, `upper`, `lower`, `trim`, `split`, `join`, `contains`, `replace`, `find`, `starts_with`, `ends_with`, `repeat`, `chars`, `lines` |
| Lists | `len`, `push`, `reverse`, `sort`, `slice`, `contains`, `range` |
| Maps | `len`, `has`, `get`, `delete`, `keys`, `values`, `contains` |
| Files | `read_file`, `write_file`, `append_file`, `file_exists`, `delete_file` |
| Program | `args` |
| Control | `error` |

Multi-word built-ins use underscores, so `starts_with` is the shape to expect
from anything added later. A local or global that reuses a built-in's name hides
it, in the type checker as well as at run time.

For user-defined records, `type(p)` returns the struct's declared name as a
string (e.g. `"Point"`), rather than a generic `"struct"` or `"object"`.

---

## Records and methods

User-defined types with named fields are declared with `struct` and constructed
like a function call. Methods are declared at the top level with receiver syntax
`fn (p: Point) method(...)` and called as `p.method(...)`:

```jin
struct Point { x: int, y: int }

# A method on Point with 'p' as the receiver
fn (p: Point) shift(dx: int, dy: int) -> Point {
    p.x = p.x + dx
    p.y = p.y + dy
    return p
}

let p = Point(10, 20)
p.shift(5, -10)
print(p)                # Point{x: 15, y: 10}
print(p.x, p.y)         # 15 10

# Bound methods are first-class values
let moveFn = p.shift
moveFn(1, 1)
print(p)                # Point{x: 16, y: 11}
```

Records can be nested, stored in arrays and maps, and even refer to their own
type recursively (e.g., `struct Node { value: int, next: Node }`). See
`examples/records.jin` for a runnable walkthrough.

---

## Maps

A map is how arbitrary key-value pairs are stored. Unlike records, map keys can
be dynamically added, queried with `has`, and deleted:

```jin
let server = {"host": "local", "port": 8080, "up": true}
print(server["host"], server["port"])   # local 8080

server["port"] = 9090                   # replace
server["tags"] = ["a", "b"]             # add
delete(server, "up")                    # remove
print(len(server), keys(server))         # 3 ["host", "port", "tags"]
```

Keys may be an `int`, a `str` or a `bool`. Entries keep the order they were
added in, so printing a map and looping over one are predictable:

```jin
let counts = {}
for w in ["pear", "apple", "pear"] {
    counts[w] = get(counts, w, 0) + 1
}
print(counts)             # {"pear": 2, "apple": 1}
for w in counts { print(w, counts[w]) }
```

Reading a key that is not there is an error rather than a quiet `nil`, which is
the same rule as an out-of-range list index:

```jin
print(server["nope"])         # error: key "nope" is not in the map
print(has(server, "nope"))    # false
print(get(server, "nope", 0)) # 0
```

`m[k] = v` changes the map in place, and so does `delete`. `sort`, `reverse`
and `slice` share that style with `push`: they return something new and leave
the original alone.

---

## Files

```jin
write_file("notes.txt", "first\nsecond\n")
append_file("notes.txt", "third\n")

for line in lines(read_file("notes.txt")) {
    print(upper(line))
}

delete_file("notes.txt")
```

Every file function takes a path as a `str` and reports the operating system's
own reason when it fails. Failures can be caught with `try / catch`, or avoided
up front using `file_exists(path)`.

---

## Error handling

Errors are handled with `try` and `catch` statements:

```jin
try {
    let text = read_file("notes.txt")
    print(text)
} catch e {
    print("failed: {e}")
}
```

The catch variable `e` is a `str` holding the exact error message that would have
ended the program. If you need to propagate an error upward, calling `error(e)`
inside the catch block re-raises it to the enclosing handler:

```jin
try {
    try {
        let val = 1 / 0
    } catch err {
        print("logging error: {err}")
        error(err)   # re-raise to outer catch
    }
} catch outer {
    print("handled: {outer}")
}
```

Everything that stops a running program can be caught:
- Explicit calls to `error("msg")`
- Failed file operations (`read_file`, `write_file`, etc.)
- Out-of-bounds array indices
- Missing map keys
- Division by zero
- Non-existent record field reads or writes

Errors can be thrown and caught across function calls, methods, and imported
modules. Type errors found by `jin check` remain strictly compile-time errors and
cannot be caught at runtime.

---

## Types are optional, but checked

Annotations are never required. When you write them, Jin enforces them before
the program runs, so mistakes are caught up front instead of halfway through a
loop:

```jin
fn area(w: int, h: int) -> int {
    return w * h
}

let width: int = 3
let height = "4"

print(area(width, height))   # error: argument 2: cannot use str where int is expected
print(area(width))           # error: area() expects 2 argument(s), got 1
print(1 + "x")               # error: cannot add int and str
if width { }                 # error: condition must be a bool, got int
```

Run `jin check file.jin` to see the problems without executing anything. Running
a file checks it first, so a program with type errors never starts.

Whatever you leave unannotated is treated as `any` and left alone, so plain
scripts keep working exactly as before. The checker also covers `let` and
assignment, function arguments and returns, loop bodies, indexing, built-in
calls, misplaced `break` / `continue` / `return`, and the expressions inside
`{interpolation}`.

The annotation names are `int`, `float`, `str`, `bool`, `nil`, `array`, `map`
and `any`. A bare `array` or `map` leaves the elements alone, which is usually
what you want, because a map holding a name and an age at once is normal:

```jin
let person: map = {"name": "Ada", "age": 36}
let mixed: array = [1, "two"]
```

---

## Closures

Functions capture the variables they use from the scope where they were
written, and the capture survives the call that created it:

```jin
fn counter() {
    let n = 0
    return fn() {
        n = n + 1
        return n
    }
}

let next = counter()
print(next(), next(), next())   # 1 2 3
```

A captured variable is the *variable*, not a snapshot of it, so two functions
made in the same scope see each other's writes:

```jin
fn running_total() {
    let total = 0
    let add = fn(n) { total = total + n }
    let show = fn() { return total }
    return [add, show]
}

let pair = running_total()
pair[0](10)
pair[0](5)
print(pair[1]())                # 15
```

A loop's variable is a fresh binding on every pass, so each function made in the
body keeps its own value instead of following the loop:

```jin
let printers = []
for word in ["one", "two", "three"] {
    printers = push(printers, fn() { return word })
}
for p in printers { print(p()) }   # one, two, three
```

Functions written in the same body may call each other in either order, because
both names are known before either body is compiled:

```jin
fn classify(n) {
    fn even(n) { if n == 0 { return true }  return odd(n - 1) }
    fn odd(n)  { if n == 0 { return false } return even(n - 1) }
    if even(n) { return "even" }
    return "odd"
}
```

`examples/closures.jin` exercises all of this. Under the hood each captured
local gets a *cell*: while the frame that owns the variable is still running the
cell points straight at the frame's stack slot, so an ordinary local read or
write is seen by every closure holding it; the cell is closed (the value copied
off the stack) when the scope ends or each time a loop starts another pass.

---

## Projects and imports

A program can be split across files. Any file can import another file beside it,
and everything in that file is reached through the name the import gave it:

```jin
# main.jin
import "utils" as u

print(u.double(21))        # 42
print(u.name)              # utils
```

```jin
# utils.jin
let name = "utils"

fn double(n: int) -> int {
    return n * 2
}
```

Every top-level `let` and `fn` in `utils.jin` is reachable as `u.<name>`, and
nothing is. Names inside a function, or inside a block, stay there — a file's
exports *are* its top-level names, so there is no `export` to write. The alias is
required, because a bare name from another file is exactly the surprise Jin is
trying to avoid; `u.helper(x)` says where `helper` came from.

An import runs the file it names, once, before the importing file's own
statements, and files are run in dependency order — so a file that imports two
files sees both of them ready. Importing the same file twice runs it once, and a
cycle is reported as `import cycle: a.jin -> b.jin -> a.jin` rather than hanging.
An import path is relative to the importing file, uses `/` between folders, and
cannot leave the folder it is in:

```jin
import "utils" as u             # utils.jin, beside this file
import "lib/math" as math       # lib/math.jin
```

A file's names are read-only from outside it: `u.name = "x"` is an error, and so
is `print(u)` — a module is a way to reach names, not a value.

Because every file's names are kept apart, an alias may not be a name the
importing file already declares, and may not shadow a built-in. Both are caught
before the program runs, at the import that caused the problem.

### Program arguments

`args()` returns the arguments the program was run with, as an `array<str>`, so
a Jin file can be a command:

```jin
let who = args()
if len(who) == 0 {
    print("no arguments given")
} else {
    print("hello, {who[0]}")
}
```

```sh
jin run greet.jin Ada Bo      # hello, Ada
```

### How a project is laid out

A folder holding a `jin.toml` is a project. The file is deliberately small — a
name, a version, and the file to start from:

```toml
# jin.toml
name = "greeting"
version = "1.0.0"
entry = "main.jin"
```

From anywhere inside the project, `jin run` runs that entry file with its
imports, `jin check` type-checks every file it reaches, and `jin build` checks
and compiles the lot:

```sh
jin new hello           # writes hello/jin.toml, hello/main.jin, hello/greeting.jin
cd hello
jin run                 # Hello, world!
jin run -- Jin          # Hello, Jin!
jin check               # hello 0.1.0: no type errors in 2 file(s)
jin build               # files, bytecode size and constants, then "ok"
```

`examples/greeting` is a project of exactly this shape. Because Jin runs from
source, `jin build` is a check-and-compile step rather than a step that writes a
binary: it proves the whole project compiles and reports what came out, and
there is no separate artefact to keep in step with the source.

---

## Architecture

### The pipeline

A language is built as a pipeline. Jin is through stage 9:

```
source text
   |  lexer.go        -> tokens
   v
tokens
   |  parser.go       -> syntax tree
   v
syntax tree            (ast.go)
   |  module.go       -> the file plus everything it imports
   v
syntax tree
   |  typechecker.go  -> type errors, or a clean bill of health
   v
syntax tree
   |  compiler.go     -> bytecode
   v
bytecode               (code.go)
   |  vm.go           -> result
   v
values                 (object.go, record.go, symboltable.go, builtins.go)

alternate back end
syntax tree
   |  interpreter.go  -> result       (jin interpret)
   v
values                 (object.go, environment.go, builtins.go)
```

The compiler resolves every name to a slot before the program runs: a global
slot, a slot in the current call frame, a captured variable, or a built-in.
That is why `compile` can show you the bytecode, and why `jin check` and
`jin compile` catch different classes of mistake.

Every instruction carries the line and column it came from, so an error thrown
by the VM points at the same place the interpreter would. When the code came
from an imported file, the instruction carries the file too, so the message reads
`utils.jin:3:5: division by zero` — the file you ran stays plain, because there
is only one of it.

### Project layout

| File | Role |
|---|---|
| `token.go` | Token types and keywords |
| `lexer.go` | Source text to tokens, with line/column tracking |
| `ast.go` | Syntax tree node definitions |
| `parser.go` | Tokens to syntax tree (Pratt parser) |
| `dump.go` | Renders a syntax tree as indented text |
| `typechecker.go` | Static types, and every check that runs before the program does |
| `template.go` | Splits `{interpolation}` inside strings, shared by the checker and compiler |
| `module.go` | Reads a file and everything it imports: paths, aliases, cycles, `jin.toml`, `jin new` |
| `code.go` | Opcodes, instruction encoding, and the disassembler behind `jin compile` |
| `symboltable.go` | Name resolution: globals, frame slots, captured variables, block scopes |
| `compiler.go` | Syntax tree to bytecode |
| `frame.go` | One active call: its closure, its position, and where its locals live |
| `vm.go` | The stack machine that runs bytecode |
| `object.go` | Runtime values, including compiled functions, closures and maps |
| `environment.go` | Variable scopes, used by the interpreter back end |
| `builtins.go` | Built-in functions, shared by both back ends |
| `interpreter.go` | Evaluates the tree, kept as a reference implementation |
| `main.go` | CLI and REPL |
| `examples/` | Programs that both engines must run identically, including the `greeting` project |

### Kept in one place

Two pieces of logic live in exactly one place so the back ends cannot drift
apart. `indexInto` and `setInto` in `vm.go` do every indexed read and write, and
`iterateValues` decides what a `for` loop walks. The interpreter calls all
three, which is why an error message about a map key is the same whichever
engine you run.

The second thing kept in one place is the capture semantics. The interpreter
already shared a captured variable, because it hands every function a pointer to
the environment it was written in; the VM matches it with cells (in `object.go`),
opened on the stack when a closure is made and closed when the scope ends. Both
engines run the same differential test corpus, so the two cannot drift apart
without a test noticing.

The third is what a file's names mean. The compiler keeps every file's globals in
the one table the VM reads, separated by a prefix the loader hands out; the
interpreter gives each file its own scope, and a field on the scope carries the
aliases so a function can use the import written above it. Both are the same rule
— a file sees its own names plainly and another file's only through an alias —
reached in the way each engine is built.

---

## Performance

Naive `fib(30)`, wall clock including start-up, compile and type-check, on the
same machine (Windows, Go 1.26, CPython 3.14):

| Engine | Time |
|---|---|
| `jin run` (bytecode VM) | ~0.40 s |
| `jin interpret` (tree walker) | ~2.4 s |
| CPython 3.14, same program | ~0.34 s |

So the VM is roughly six times faster than Jin's own tree-walking back end, and
in the same ballpark as CPython on a call-heavy benchmark. Getting clearly past
CPython would need unboxed stack slots and inline caches, which are a later
stage's work.

The VM keeps that lead by resolving names to slots before running, pooling small
integers, booleans and `nil` instead of allocating them, reusing call frames,
and handling the common arithmetic cases without building any objects that get
thrown away.

---

## Testing

```sh
go test ./...      # the whole suite
go vet ./...       # static checks
gofmt -l .         # formatting (prints nothing when clean)
```

354 test cases pass, covering the lexer, parser, type checker, compiler,
interpreter, the VM, the runtime value model, modules and projects, and every
example program.

The centrepiece is the **differential corpus**: each program is run on the
bytecode VM and on the tree-walking interpreter, and the two results — including
error text — have to match exactly. Error programs take the same route, which is
what keeps the two engines honest about their messages rather than merely
compatible. Projects get the same treatment: the tests write real files into a
temporary folder and require both back ends to agree on what the whole project
prints, and on where it fails when a module, an alias or an import path is wrong.

---

## Project status

- [x] Stage 1: lexer
- [x] Stage 2: parser
- [x] Stage 3: interpreter
- [x] Stage 4: type checker
- [x] Stage 5: bytecode compiler + VM
- [x] Stage 6: standard library
- [x] Closures: shared captures, loop-pass bindings, and mutual recursion between nested functions
- [x] Stage 7: package manager + CLI — imports, projects, `jin new` / `jin build`, `args()`
- [x] Stage 8: records — user-defined struct types, constructors, receiver methods (`fn (p: Point)`), dotted field access and mutation
- [x] Stage 9: error handling — try / catch statements, catchable runtime errors and error(), re-raising

Current version: `0.9.0 (stage 9: error handling)` — see `jin --version`.

### What is in, and what is not

In: records, try / catch error handling, maps, `sort` / `reverse` / `slice`, the string helpers, the number helpers,
`error`, and file reading and writing. `examples/records.jin`, `examples/errors.jin`, `examples/collections.jin` and
`examples/files.jin` exercise all of it. Multi-file programs are in too:
`import "utils" as u` puts another file's top-level names behind an alias, a
folder with a `jin.toml` is a project, and `jin new`, `jin build`, `jin check`
and `jin run` work on one. `examples/greeting` is a two-file project.

Not in, and honest about it:

- No remote packages. Imports are files beside the one that imports them; there
  is no registry, no download step, and no lock file, because there is nothing
  to fetch yet. A project is a folder and nothing else.
- No compiled output. `jin build` checks and compiles a project and reports what
  came out, but the result is not written to disk, because Jin runs from source
  and a stale binary is worse than no binary.
- No `finally` blocks, no custom error types, and no `try` as an expression. Catch
  blocks receive a `str` error message.
- No default field values, no named construction (`Point{x: 1}`), no
  inheritance or interfaces. Fields must all be passed in declaration order
  when calling `Point(...)`.
- No user-supplied comparison for `sort`. It orders numbers or strings, and
  refuses an array that mixes them instead of guessing.
- No `input` from the terminal yet; the REPL is the only interactive path.
- No string literals spanning lines.

### Roadmap

- **Stage 10 — remote packages:** a registry and a versioned lock file, so
  `import "json" as json` can pull a package instead of a folder. The loader in
  `module.go` is where that lands: it already decides what a path means, what a
  project is, and how files depend on each other.
- **Later — a faster VM:** unboxed stack slots and inline caches, to move clearly
  ahead of CPython rather than alongside it.

---

## Developer

**JOJIN JOHN** — designed and written from scratch.

- GitHub: [@jojin1709](https://github.com/jojin1709)
- Repository: [jojin1709/jin](https://github.com/jojin1709/jin)

Every stage of this project — the lexer, the parser, the type checker, the
bytecode compiler, the virtual machine, the standard library, the package
manager and CLI, and the test suite — was implemented by hand in Go, without a
parser generator or a third-party library.

---

## License

MIT. See [LICENSE](LICENSE) for the full text.

Copyright © 2026 JOJIN JOHN.

<div align="center">

**Built by [JOJIN JOHN](https://github.com/jojin1709)**

</div>
