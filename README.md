# Jin

Jin is a small, fast programming language designed to be as easy to write as
Python but with fewer footguns and much better performance.

## Design rules

1. **Braces, not invisible whitespace.** No broken programs because of a stray tab.
2. **One obvious way to do things.** No decorators, no metaclasses, no dunder magic.
3. **No silent type mixing.** `1 + 1.5` is an error, not a surprise. Convert with `int()` or `float()`.
4. **Conditions must be `bool`.** `if 1 { }` is rejected instead of quietly being truthy.
5. **No accidental globals.** Assigning to a name that was never declared is an error.
6. **Fast by default.** Source compiles to bytecode and runs on a stack VM. The
   tree-walking interpreter is still there for cross-checking, but it is not
   what `run` uses.

## Example

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

## Usage

```sh
go build -o jin.exe .

./jin.exe examples/tour.jin        # run a program (bytecode VM)
./jin.exe examples/soak.jin        # a bigger program: closures, recursion, loops
./jin.exe examples/types.jin       # run the type-checked demo
./jin.exe interpret examples/tour.jin # run on the tree-walking interpreter
./jin.exe check examples/tour.jin  # type-check without running
./jin.exe compile examples/tour.jin # print the bytecode
./jin.exe parse examples/tour.jin  # print the syntax tree
./jin.exe tokens examples/tour.jin # print the token stream
./jin.exe                           # REPL
go test ./...
```

Stages 1, 2, 4 and 5 (`tokens`, `parse`, `check`, `compile`) are available on
their own, so you can watch each layer of the pipeline separately. `run` and
`interpret` must agree on every program, and the test suite checks exactly
that.

## Language tour

```jin
# comments start with a hash

let x = 5                  # let declares
let name = "Jin"
x = x + 1                  # assign to an existing name

fn add(a, b) { return a + b }        # implicit return also works: fn sq(x) { x * x }

if x > 5 { print("big") } else if x == 5 { print("five") } else { print("small") }

for n in [1, 2, 3] { print(n) }      # also: for ch in "abc", for i in range(10)
while x > 0 { x = x - 1; if x == 2 { break } }

let f = fn(v) { v * 2 }              # anonymous functions
print("{name} has {len(name)} letters")   # string interpolation
print("use {{braces}} for literal braces")
```

Built-ins: `print`, `len`, `push`, `str`, `int`, `float`, `type`, `range`,
`upper`, `lower`, `trim`, `split`, `join`, `contains`, `abs`, `min`, `max`.

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

## How it works

A language is built as a pipeline. Jin is through stage 5:

```
source text
   |  lexer.go        -> tokens
   v
tokens
   |  parser.go       -> syntax tree
   v
syntax tree            (ast.go)
   |  typechecker.go  -> type errors, or a clean bill of health
   v
syntax tree
   |  compiler.go     -> bytecode
   v
bytecode               (code.go)
   |  vm.go           -> result
   v
values                 (object.go, symboltable.go, builtins.go)

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
by the VM points at the same place the interpreter would.

| File | Role |
|---|---|
| `token.go` | Token types and keywords |
| `lexer.go` | Source text to tokens, with line/column tracking |
| `ast.go` | Syntax tree node definitions |
| `parser.go` | Tokens to syntax tree (Pratt parser) |
| `dump.go` | Renders a syntax tree as indented text |
| `typechecker.go` | Static types, and every check that runs before the program does |
| `template.go` | Splits `{interpolation}` inside strings, shared by the checker and compiler |
| `code.go` | Opcodes, instruction encoding, and the disassembler behind `jin compile` |
| `symboltable.go` | Name resolution: globals, frame slots, captured variables, block scopes |
| `compiler.go` | Syntax tree to bytecode |
| `frame.go` | One active call: its closure, its position, and where its locals live |
| `vm.go` | The stack machine that runs bytecode |
| `object.go` | Runtime values, including compiled functions and closures |
| `environment.go` | Variable scopes, used by the interpreter back end |
| `builtins.go` | Built-in functions, shared by both back ends |
| `interpreter.go` | Evaluates the tree, kept as a reference implementation |
| `main.go` | CLI and REPL |

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

Two known limits, both of which the test suite documents rather than hides:

- A local variable captured by *two* sibling functions is copied once per
  function, so writes through one are not seen by the other. Uses where a
  single closure mutates its own capture, including the counter above, work.
- Two functions nested inside another function cannot call each other, because
  each is compiled before the other's slot is filled. Functions declared at the
  top level have no such problem, so mutual recursion belongs there.

## Speed

Naive `fib(30)`, wall clock including start-up, compile and type-check, on the
same machine (Windows, Go 1.26, CPython 3.13):

| Engine | Time |
|---|---|
| `jin run` (bytecode VM) | ~0.42 s |
| `jin interpret` (tree walker) | ~2.2 s |
| CPython 3.13, same program | ~0.34 s |

So the VM is a little over five times faster than Jin's own tree-walking back
end, and in the same ballpark as CPython on a call-heavy benchmark. Getting
clearly past CPython would need unboxed stack slots and inline caches, which
are a later stage's work.

The VM keeps that lead by resolving names to slots before running, pooling
small integers, booleans and `nil` instead of allocating them, reusing call
frames, and handling the common arithmetic cases without building any objects
that get thrown away.


## Status

- [x] Stage 1: lexer
- [x] Stage 2: parser
- [x] Stage 3: interpreter
- [x] Stage 4: type checker
- [x] Stage 5: bytecode compiler + VM
- [ ] Stage 6: standard library
- [ ] Stage 7: package manager + CLI

## License

MIT
