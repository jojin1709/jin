# Jin

Jin is a small, fast programming language designed to be as easy to write as
Python but with fewer footguns and much better performance.

## Design rules

1. **Braces, not invisible whitespace.** No broken programs because of a stray tab.
2. **One obvious way to do things.** No decorators, no metaclasses, no dunder magic.
3. **No silent type mixing.** `1 + 1.5` is an error, not a surprise. Convert with `int()` or `float()`.
4. **Conditions must be `bool`.** `if 1 { }` is rejected instead of quietly being truthy.
5. **No accidental globals.** Assigning to a name that was never declared is an error.
6. **A missing key is an error.** `m["nope"]` is not quietly `nil`; ask with `has(m, "nope")`.
7. **Fast by default.** Source compiles to bytecode and runs on a stack VM. The
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
./jin.exe examples/collections.jin # maps, lists and the built-ins for them
./jin.exe examples/types.jin       # run the type-checked demo
./jin.exe examples/files.jin       # reading and writing files
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
that: every program in the differential corpus, every error message, and every
file in `examples/` has to produce byte-identical output on both back ends.

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

let person = {"name": "Ada", "age": 36}   # a map
print(person["name"], has(person, "job")) # Ada false
for field in person { print(field) }      # maps loop over their keys
```

Built-ins:

| Group | Functions |
|---|---|
| Output | `print` |
| Numbers | `abs`, `min`, `max`, `floor`, `ceil`, `round`, `sqrt`, `pow`, `random`, `int`, `float`, `str`, `type` |
| Strings | `len`, `upper`, `lower`, `trim`, `split`, `join`, `contains`, `replace`, `find`, `starts_with`, `ends_with`, `repeat`, `chars`, `lines` |
| Lists | `len`, `push`, `reverse`, `sort`, `slice`, `contains`, `range` |
| Maps | `len`, `has`, `get`, `delete`, `keys`, `values`, `contains` |
| Files | `read_file`, `write_file`, `append_file`, `file_exists`, `delete_file` |
| Control | `error` |

Multi-word built-ins use underscores, so `starts_with` is the shape to expect
from anything added later.

## Maps

A map is the way to write a value with named fields, and it is the only one, so
its values may have different types:

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
own reason when it fails. Because Jin has no `try` yet, a failure stops the
program, so `file_exists(path)` is how you ask first.

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

## How it works

A language is built as a pipeline. Jin is through stage 6:

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
| `object.go` | Runtime values, including compiled functions, closures and maps |
| `environment.go` | Variable scopes, used by the interpreter back end |
| `builtins.go` | Built-in functions, shared by both back ends |
| `interpreter.go` | Evaluates the tree, kept as a reference implementation |
| `main.go` | CLI and REPL |

Two pieces of logic live in exactly one place so the back ends cannot drift
apart: `indexInto` and `setInto` in `vm.go` do every indexed read and write,
and `iterateValues` decides what a `for` loop walks. The interpreter calls all
three, which is why an error message about a map key is the same whichever
engine you run.

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
- [x] Stage 6: standard library
- [ ] Stage 7: package manager + CLI

### Stage 6: what is in, and what is not

In: maps, `sort` / `reverse` / `slice`, the string helpers, the number helpers,
`error`, and file reading and writing. `examples/collections.jin` and
`examples/files.jin` exercise all of it.

Not in, and honest about it:

- No `try` / `catch`, so `error(...)` and a failed `read_file` end the program.
  `file_exists` and `has` are how you check before acting.
- No user-supplied comparison for `sort`. It orders numbers or strings, and
  refuses an array that mixes them instead of guessing.
- No user-defined types or records. A map is the record type, which is why a
  map's values are allowed to differ in type while an array's are not.
- No `input` from the terminal yet; the REPL is the only interactive path.

## License

MIT
