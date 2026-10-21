package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------- helpers

func parseOf(t *testing.T, src string) *Program {
	t.Helper()
	p := NewParser(NewLexer(src))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors for %q: %s", src, strings.Join(p.Errors(), "; "))
	}
	return program
}

func checkOf(t *testing.T, src string) []string {
	t.Helper()
	return Check(parseOf(t, src))
}

func expectClean(t *testing.T, src string) {
	t.Helper()
	if errs := checkOf(t, src); len(errs) > 0 {
		t.Errorf("expected no type errors for:\n%s\ngot: %s", src, strings.Join(errs, "; "))
	}
}

func expectTypeError(t *testing.T, src, wantSub string) {
	t.Helper()
	errs := checkOf(t, src)
	if len(errs) == 0 {
		t.Errorf("expected a type error containing %q for:\n%s\ngot none", wantSub, src)
		return
	}
	joined := strings.Join(errs, "; ")
	if !strings.Contains(joined, wantSub) {
		t.Errorf("for:\n%s\nwant an error containing %q\ngot: %s", src, wantSub, joined)
	}
}

// ---------------------------------------------------------------- tests

func TestValidProgramsPassTheChecker(t *testing.T) {
	programs := []string{
		`let x: int = 5`,
		`let name: str = "Jin"
let greeting = "Hello, {name}!"`,
		`fn add(a: int, b: int) -> int { return a + b }
add(1, 2)`,
		`fn fib(n) {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}
fib(10)`,
		`let xs = [1, 2, 3]
let total = 0
for x in xs { total = total + x }
total`,
		`let n = 0
while n < 5 {
    n = n + 1
    if n == 3 { continue }
    if n == 6 { break }
}
n`,
		`let f = fn(v: int) -> int { v * 2 }
f(21)`,
		`let s = ""
for ch in "abc" { s = s + ch }
s`,
		`print("fib: {len([1, 2, 3])}")`,
		`let a = [1, 2]
a[0] = 99
a[1]`,
		`fn outer() {
    fn inner(x: int) -> int { return x + 1 }
    return inner(1)
}
outer()`,
		// A closure may assign to a local of the enclosing function.
		`fn counter() {
    let n = 0
    let inc = fn() { n = n + 1; return n }
    return inc
}
let next = counter()
next()`,
		// A bare "array" annotation is allowed.
		`let xs: array = []
xs = push(xs, 1)
for x in xs { print(x) }`,
		// Recursion against an annotated signature.
		`fn fact(n: int) -> int {
    if n <= 1 { return 1 }
    return n * fact(n - 1)
}
fact(5)`,
		// push keeps the element type it knows about.
		`let xs = [1, 2]
let ys = push(xs, 3)
for y in ys { print(y + 1) }`,
		`let grid = [1, 2, 3]
grid[1] = grid[0] + grid[2]
grid`,
		// Maps are the record type, so their values may differ in type.
		`let person = {"name": "Ada", "age": 36}
print(person["name"], person["age"], len(person))`,
		`let m = {}
m["a"] = 1
m["b"] = 2
has(m, "a")`,
		`let m: map = {"a": 1}
for k in m { print(k) }`,
		`let m = {1: "one", true: 3}
get(m, 1, "none")`,
		`let xs = [3, 1, 2]
sort(xs)
reverse(xs)
slice(xs, 0, 2)
min(xs)
max(xs)
min(1, 2)`,
		`replace("a-b", "-", "+")
find("abc", "b")
starts_with("abc", "a")
ends_with("abc", "c")
repeat("ab", 2)
chars("abc")
lines("a\nb")`,
		`floor(1.5)
ceil(1.5)
round(1.5)
sqrt(4)
pow(2, 3)
pow(2, 0.5)
abs(-1)`,
	}
	for _, src := range programs {
		expectClean(t, src)
	}
}

func TestMapTypeChecking(t *testing.T) {
	// A map is keyed by int, str or bool and by nothing else.
	expectTypeError(t, `let m = {}
m[[1]] = 2`, "map keys must be int, str or bool")
	expectTypeError(t, `let m = {}
print(m[[1]])`, "map keys must be int, str or bool")
	expectTypeError(t, `let m = {}
has(m, [1])`, "map keys must be int, str or bool")

	// A key may be any of the three, so mixing them in one map is fine.
	expectClean(t, `let m = {"a": 1}
print(m[1], m["a"], m[true])`)

	// "map" is a known annotation, and a map is not an array.
	expectClean(t, `let m: map = {}`)
	expectTypeError(t, `let m: int = {}`, "cannot assign map to int")

	// len() takes a map, and the map helpers insist on a map.
	expectClean(t, `len({})`)
	expectTypeError(t, `has("abc", "a")`, "has() expects map")
	expectTypeError(t, `keys([1, 2])`, "keys() expects map")
	expectTypeError(t, `values([1, 2])`, "values() expects map")
	expectTypeError(t, `get([1], 0, 1)`, "get() expects map")
	expectTypeError(t, `delete([1], 0)`, "delete() expects map")

	// Anything that is not iterable is still refused.
	expectTypeError(t, `for k in 5 { print(k) }`, "cannot loop over int")
	expectClean(t, `for k in {} { print(k) }`)
}

func TestFileBuiltinsAreChecked(t *testing.T) {
	expectClean(t, `read_file("a.txt")`)
	expectClean(t, `write_file("a.txt", "text")`)
	expectClean(t, `append_file("a.txt", "more")`)
	expectClean(t, `file_exists("a.txt")`)
	expectClean(t, `delete_file("a.txt")`)
	expectClean(t, `let text: str = read_file("a.txt")`)

	expectTypeError(t, `read_file(1)`, "read_file() expects a path as str")
	expectTypeError(t, `write_file(1, "x")`, "write_file() argument 1 expects str")
	expectTypeError(t, `write_file("a", 1)`, "write_file() argument 2 expects str")
	expectTypeError(t, `append_file("a")`, "append_file() takes 2 argument(s)")
	expectTypeError(t, `file_exists(1)`, "file_exists() expects a path as str")
	expectTypeError(t, `delete_file(1)`, "delete_file() expects a path as str")
}

func TestCollectionBuiltinsAreChecked(t *testing.T) {
	expectTypeError(t, `sort(5)`, "sort() expects array")
	expectTypeError(t, `reverse(5)`, "reverse() expects array or str")
	expectTypeError(t, `slice(5, 0, 1)`, "slice() expects array or str")
	expectTypeError(t, `slice([1], 0, "x")`, "slice() expects int bounds")
	expectTypeError(t, `replace("a", "b", 1)`, "replace() argument 3 expects str")
	expectTypeError(t, `find("a", 1)`, "find() argument 2 expects str")
	expectTypeError(t, `starts_with("a", 1)`, "starts_with() argument 2 expects str")
	expectTypeError(t, `repeat(1, 2)`, "repeat() expects str")
	expectTypeError(t, `floor("x")`, "floor() expects int or float")
	expectTypeError(t, `sqrt("x")`, "sqrt() expects int or float")
	expectTypeError(t, `pow(1)`, "pow() takes 2 argument(s)")
	expectTypeError(t, `error()`, "error() takes 1 argument(s)")
	expectTypeError(t, `random(1, 2, 3)`, "random() takes 0, 1 or 2 argument(s)")
}

func TestBasicTypeMismatches(t *testing.T) {
	cases := []struct{ src, want string }{
		{`let x: int = "hello"`, "cannot assign str to int"},
		{`let x: int = 5` + "\n" + `x = "s"`, "cannot assign str to int"},
		{`1 + "x"`, "cannot add int and str"},
		{`"a" - "b"`, "cannot use '-' on str and str"},
		{`1 + 1.5`, "convert one explicitly"},
		{`let n = 1` + "\n" + `n * 2.5`, "convert one explicitly"},
		{`!5`, "'!' expects a bool"},
		{`-"text"`, "'-' expects a number"},
		{`true && 1`, "'&&' expects bool operands"},
		{`1 < "a"`, "cannot compare"},
		{`if 1 { print("x") }`, "condition must be a bool"},
		{`while "yes" { }`, "condition must be a bool"},
		{`let x: int = 5` + "\n" + `x + true`, "cannot add int and bool"},
	}
	for _, c := range cases {
		expectTypeError(t, c.src, c.want)
	}
}

func TestUnknownTypeName(t *testing.T) {
	expectTypeError(t, `let x: integer = 5`, `unknown type "integer"`)
	expectTypeError(t, `fn f(a: string) { }`, `unknown type "string"`)
}

func TestFunctionCallsAreChecked(t *testing.T) {
	expectTypeError(t, "fn f(a: int) { }\nf(1, 2)", "f() expects 1 argument(s), got 2")
	expectTypeError(t, "fn f(a: int) { }\nf()", "f() expects 1 argument(s), got 0")
	expectTypeError(t, `fn f(a: int) { }`+"\n"+`f("x")`, "argument 1: cannot use str where int is expected")
	expectTypeError(t, `let x = 5`+"\n"+`x(1)`, "is not callable")
	expectTypeError(t, `fn f(a: str, b: bool) { }`+"\n"+`f("ok", 1)`, "argument 2")
}

func TestReturnTypesAreChecked(t *testing.T) {
	expectTypeError(t, `fn f() -> int { return "text" }`, "cannot return str")
	expectTypeError(t, `fn f() -> str { return 42 }`, "cannot return int")
	// Implicit return of the last expression must match too.
	expectTypeError(t, `fn f() -> int { "just a string" }`, "declared to return int")
	// Explicitly returning the right type is fine.
	expectClean(t, `fn f() -> int { return 42 }`)
	expectClean(t, `fn f() -> int { 7 }`)
}

func TestControlFlowPlacement(t *testing.T) {
	expectTypeError(t, `break`, "'break' is only allowed inside a loop")
	expectTypeError(t, `continue`, "'continue' is only allowed inside a loop")
	expectTypeError(t, `return 1`, "'return' is only allowed inside a function")
	expectTypeError(t, `while true { fn g() { break } }`, "'break' is only allowed inside a loop")
	expectClean(t, `while true { break }`)
	expectClean(t, `fn f() { return 1 }`)
}

func TestUndefinedVariables(t *testing.T) {
	expectTypeError(t, `undefinedThing`, `undefined variable "undefinedThing"`)
	expectTypeError(t, `print(x)`+"\n"+`let x = 5`, `undefined variable "x"`)
	expectTypeError(t, `typo = 5`, `use 'let' to declare it first`)
}

// A helper at the top of a file may use a variable declared further down,
// because the file has to finish loading before anything runs.
func TestFunctionsCanSeeVariablesDeclaredLater(t *testing.T) {
	expectClean(t, `fn shout() { print(upper(message)) }
let message = "hi"
shout()`)

	expectClean(t, `fn bump() { total = total + 1 }
let total = 0
bump()
bump()
print(total)`)

	// Top-level code does not get that allowance.
	expectTypeError(t, `print(message)`+"\n"+`let message = "hi"`, `undefined variable "message"`)
}

// Two functions written in the same body can call each other in either order,
// because the checker declares them all before it checks any of their bodies.
func TestNestedFunctionsCanCallEachOther(t *testing.T) {
	expectClean(t, `fn outer() {
    fn a(n) {
        if n <= 0 { return "a" }
        return b(n - 1)
    }
    fn b(n) {
        if n <= 0 { return "b" }
        return a(n - 1)
    }
    return a(5)
}
print(outer())`)

	expectClean(t, `fn outer() {
    fn first() { return second() }
    fn second() { return "second" }
    return first()
}
print(outer())`)

	// A local declared with 'let' still has to come first, nested or not.
	expectTypeError(t, `fn outer() {
    fn a() { return later }
    let later = 1
    return a()
}
print(outer())`, `undefined variable "later"`)

	// The allowance stops at the block the functions are declared in.
	expectTypeError(t, `if true {
    fn hidden() { return 1 }
}
hidden()`, `undefined variable "hidden"`)
}

// A variable with the same name as a built-in hides it, exactly as it does at
// run time, so built-in names are still usable as ordinary variables.
func TestLocalsShadowBuiltins(t *testing.T) {
	expectClean(t, `fn make() {
    let get = fn(x) { return x * 2 }
    return get(21)
}
print(make())`)

	expectClean(t, `fn width(items) {
    let len = 3
    return len + 1
}
print(width([]))`)

	// Nothing hides the built-in here, so its own checks still apply.
	expectTypeError(t, `len(5)`, "len() expects str, array or map")
}

func TestLoopingOverNonIterable(t *testing.T) {
	expectTypeError(t, `for x in 5 { }`, "cannot loop over int")
	expectTypeError(t, `for x in true { }`, "cannot loop over bool")
	expectClean(t, `for x in [1, 2] { }`)
	expectClean(t, `for ch in "abc" { }`)
}

func TestIndexingIsChecked(t *testing.T) {
	expectTypeError(t, `let x = 5`+"\n"+`x[0]`, "cannot index int")
	expectTypeError(t, `let a = [1, 2]`+"\n"+`a["zero"]`, "index must be an int")
	expectClean(t, `[1, 2][0]`)
	expectClean(t, `"abc"[1]`)
}

func TestBuiltinSignaturesAreChecked(t *testing.T) {
	expectTypeError(t, `len(5)`, "len() expects str, array or map")
	expectTypeError(t, `len("a", "b")`, "len() takes 1 argument")
	expectTypeError(t, `upper(5)`, "upper() expects str")
	expectTypeError(t, `push([1], 2, 3)`, "push() takes 2 argument(s)")
	expectTypeError(t, `join("a,b", "-")`, "join() expects array")
	expectTypeError(t, `range("ten")`, "range() expects int")
	expectClean(t, `len("abc")`)
	expectClean(t, `join(split("a,b", ","), "-")`)
	expectClean(t, `print(1, "two", true)`)
	expectClean(t, `min(1, 2, 3)`)
	expectClean(t, `abs(-1.5)`)
}

func TestInterpolationIsTypeChecked(t *testing.T) {
	expectTypeError(t, `"value is {undefinedThing}"`, `undefined variable "undefinedThing"`)
	expectTypeError(t, `"bad {1 + \"x\"}"`, "cannot add int and str")
	expectTypeError(t, `"bad {len(5)}"`, "len() expects str, array or map")
	expectClean(t, `let n = 5`+"\n"+`"n is {n}, doubled {n * 2}"`)
	expectClean(t, `"{{literal braces}}"`)
	expectClean(t, `let xs = [1, 2]`+"\n"+`"count {len(xs)}"`)
}

// 'nil' is a keyword, but it is also the name of a type.
func TestNilIsAUsableAnnotation(t *testing.T) {
	expectClean(t, `let x: nil = nil`)
	expectClean(t, `fn nothing() -> nil { return nil }`)
}

// An index that is written out in full is checked before the program runs.
func TestConstantIndexIsChecked(t *testing.T) {
	expectClean(t, `print([1, 2, 3][0])`)
	expectClean(t, `print([1, 2, 3][2])`)
	expectTypeError(t, `print([1, 2, 3][3])`, "index 3 out of range (length 3)")
	expectTypeError(t, `print([1, 2, 3][-1])`, "index -1 out of range (length 3)")
	// An index that is not written out in full is a run-time check.
	expectClean(t, `let i = 9`+"\n"+`print([1, 2, 3][i])`)
}

func TestArrayElementTypesAreUnified(t *testing.T) {
	expectClean(t, `let mixed = [1, "two"]`)
	// Annotated arrays are enforced through push.
	expectTypeError(t, `let xs: int = [1, 2]`, "cannot assign array<int> to int")
	expectClean(t, `let xs = [1, 2]`+"\n"+`for x in xs { print(x + 1) }`)
}

func TestErrorPositionsAreReported(t *testing.T) {
	errs := checkOf(t, "let a = 1\nlet b: str = 2")
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}
	if !strings.HasPrefix(errs[0], "2:") {
		t.Errorf("error should point at line 2, got %q", errs[0])
	}
}

func TestInterpolationErrorsPointAtTheRealLine(t *testing.T) {
	src := "let a = 1\nlet b = 2\nprint(\"total {a + \\\"x\\\"}\")"
	errs := checkOf(t, src)
	if len(errs) == 0 {
		t.Fatalf("expected an interpolation error, got none")
	}
	if !strings.HasPrefix(errs[0], "3:") {
		t.Errorf("interpolation error should point at line 3, got %q", errs[0])
	}
}

func TestErrorsAreSortedByPosition(t *testing.T) {
	src := "let a: int = \"x\"\nlet b: int = \"y\"\nlet c: int = \"z\""
	errs := checkOf(t, src)
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	for i, want := range []string{"1:", "2:", "3:"} {
		if !strings.HasPrefix(errs[i], want) {
			t.Errorf("error %d should start with %q, got %q", i, want, errs[i])
		}
	}
}

// The REPL checks one line at a time, so the checker has to remember what
// earlier lines declared.
func TestCheckerRemembersTypesAcrossCalls(t *testing.T) {
	c := NewChecker()

	if errs := c.CheckProgram(parseOf(t, `let x: int = 5`)); len(errs) > 0 {
		t.Fatalf("first chunk should be clean, got %v", errs)
	}

	if errs := c.CheckProgram(parseOf(t, `let y: str = x`)); len(errs) == 0 {
		t.Fatal("expected an error: x was declared int on the previous line")
	}

	if errs := c.CheckProgram(parseOf(t, `fn twice(n: int) -> int { return n * 2 }`)); len(errs) > 0 {
		t.Fatalf("defining a function should be clean, got %v", errs)
	}

	if errs := c.CheckProgram(parseOf(t, `twice("no")`)); len(errs) == 0 {
		t.Fatal("expected an argument error for twice(\"no\")")
	}

	if errs := c.CheckProgram(parseOf(t, `twice(21)`)); len(errs) > 0 {
		t.Fatalf("twice(21) should be clean, got %v", errs)
	}
}

// A failed REPL line must not leave its declarations behind.
func TestScopeRollbackUndoesFailedLine(t *testing.T) {
	c := NewChecker()

	before := c.GlobalScope()
	if errs := c.CheckProgram(parseOf(t, `let y: str = 1`)); len(errs) == 0 {
		t.Fatal("expected a type error")
	}
	c.SetGlobalScope(before)

	// y was rolled back, so using it is now an undefined variable again.
	errs := c.CheckProgram(parseOf(t, `y`))
	if len(errs) == 0 || !strings.Contains(errs[0], "undefined variable") {
		t.Fatalf("expected y to be undefined after rollback, got %v", errs)
	}
}

func TestStructTypeChecking(t *testing.T) {
	// Valid programs
	expectClean(t, `struct Point { x: int, y: int }
let p = Point(1, 2)
p.x = 10
let px: int = p.x`)

	expectClean(t, `struct Node { value: int, next: Node }
let n = Node(1, Node(2, nil))
let v: int = n.value`)

	expectClean(t, `struct User { name: str, active: bool }
fn check(u: User) -> bool {
    return u.active
}`)

	// Errors
	expectTypeError(t, `struct Point { x: int, x: str }`, "duplicate field \"x\"")
	expectTypeError(t, `struct Point { x: UnknownType }`, "unknown type \"UnknownType\"")
	expectTypeError(t, `struct Point { x: int, y: int }
Point(1)`, "expects 2 value(s), got 1")
	expectTypeError(t, `struct Point { x: int, y: int }
Point(1, "bad")`, "field \"y\": cannot use str where int is expected")
	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point(1, 2)
p.z`, "has no field \"z\"")
	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point(1, 2)
p.x = "str"`, "cannot assign str to int field \"x\"")
	expectTypeError(t, `struct Point { x: int }
Point.x`, "cannot read a field of the type Point")
	expectTypeError(t, `struct Point { x: int }
Point.x = 1`, "cannot assign a field of the type Point")
	expectTypeError(t, `let m = {"a": 1}
m.a`, "cannot read a field of map; use m[\"a\"]")
	expectTypeError(t, `let x = 42
x.a`, "cannot read a field of int")
}

func TestRecordMethodsTypeChecking(t *testing.T) {
	// Valid method definitions and calls
	expectClean(t, `struct Point { x: int, y: int }
fn (p: Point) sum() -> int {
    return p.x + p.y
}
let pt = Point(3, 4)
let total: int = pt.sum()`)

	expectClean(t, `struct Counter { count: int }
fn (c: Counter) bump(amount: int) -> int {
    c.count = c.count + amount
    return c.count
}
let c = Counter(0)
c.bump(5)`)

	expectClean(t, `struct Point { x: int, y: int }
fn (p: Point) clone() -> Point {
    return Point(p.x, p.y)
}
let p = Point(1, 2)
let p2: Point = p.clone()`)

	// Errors
	expectTypeError(t, `fn (p: Unknown) foo() {}`, "cannot define method on unknown type \"Unknown\"")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) x() {}`, "method \"x\" clashes with field of struct Point")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) foo() {}
fn (p: Point) foo() {}`, "duplicate method \"foo\" on struct Point")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) add(a: int) -> int { return p.x + a }
let p = Point(1)
p.add()`, "expects 1 argument(s), got 0")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) add(a: int) -> int { return p.x + a }
let p = Point(1)
p.add("not an int")`, "cannot use str where int is expected")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) length() -> int { return p.x }
let p = Point(1)
p.length = 10`, "cannot assign to method \"length\" of Point")
	expectTypeError(t, `struct Point { x: int }
fn (p: Point) badReturn() -> int {
    return "not an int"
}`, "cannot return str from a function declared to return int")
}

func TestTypeCheckTryCatch(t *testing.T) {
	expectClean(t, `try {
    let x: int = 10
} catch e {
    let msg: str = e
    print(msg)
}`)

	expectTypeError(t, `try {
    let x = 1
} catch print {
}`, `catch variable "print" cannot shadow a builtin`)

	expectTypeError(t, `try {
    let x = 1
} catch len {
}`, `catch variable "len" cannot shadow a builtin`)

	expectTypeError(t, `try {
    let x = 1
} catch e {
    let msg = e
}
print(e)`, `undefined variable "e"`)

	expectTypeError(t, `try {
    let x = 1
} catch e {
    let n: int = e
}`, `cannot assign str to int variable "n"`)

	expectTypeError(t, `try {
    let n: int = "hello"
} catch e {
}`, `cannot assign str to int variable "n"`)

	expectTypeError(t, `try {
    let n = 1
} catch e {
    let m: int = "bad"
}`, `cannot assign str to int variable "m"`)

	expectClean(t, `try {
    error("failed")
} catch e {
    error(e)
}`)
}

func TestTypeCheckStage10(t *testing.T) {
	// 1. Struct literals
	expectClean(t, `struct Point { x: int, y: int }
let p1 = Point{x: 1, y: 2}
let p2 = Point{y: 20, x: 10}`)

	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point{x: 1, z: 2}`, `Point has no field "z"`)

	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point{x: 1}`, `missing field "y" in Point`)

	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point{x: 1, x: 2, y: 3}`, `duplicate field "x" in Point`)

	expectTypeError(t, `struct Point { x: int, y: int }
let p = Point{x: "bad", y: 2}`, `field "x": cannot use str where int is expected`)

	// 2. Builtins
	expectClean(t, `let s: str = input("name: ")
let all_s: str = read_all_stdin()
let m = map([1, 2, 3], fn(x) { return x * 2 })
let f = filter([1, 2, 3], fn(x) { return x > 1 })
let r: int = reduce([1, 2, 3], fn(acc, x) { return acc + x }, 0)
let a: bool = any([1, 2, 3], fn(x) { return x > 2 })
let al: bool = all([1, 2, 3], fn(x) { return x > 0 })
let sb = sort_by([3, 1, 2], fn(a, b) { return a - b })
let idx: int = find_index([1, 2, 3], fn(x) { return x == 2 })`)

	expectTypeError(t, `input(123)`, `input() expects str, got int`)
	expectTypeError(t, `read_all_stdin("extra")`, `read_all_stdin() takes 0 argument(s), got 1`)
}
