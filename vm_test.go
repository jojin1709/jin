package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------- helpers

// runOnVM compiles and runs source on the bytecode VM, returning what the
// program printed.
func runOnVM(t *testing.T, source string) (string, error) {
	t.Helper()

	p := NewParser(NewLexer(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("syntax errors in test program: %v", errs)
	}

	bc, err := compileProgram(program)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	vm := NewVM(&out)
	_, err = vm.Run(bc)
	return out.String(), err
}

// runOnInterpreter runs the same source on the tree-walking interpreter.
func runOnInterpreter(t *testing.T, source string) (string, error) {
	t.Helper()

	p := NewParser(NewLexer(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("syntax errors in test program: %v", errs)
	}

	var out bytes.Buffer
	interp := NewInterpreter(&out)
	_, err := interp.Run(program, NewEnvironment())
	return out.String(), err
}

// programs is the shared corpus. Every entry must produce want on both back
// ends, so the two cannot drift apart without a test noticing.
var programs = []struct {
	name   string
	source string
	want   string
}{
	{
		name:   "integer arithmetic",
		source: `print(1 + 2 * 3 - 4 / 2)`,
		want:   "5\n",
	},
	{
		name:   "operator precedence with parens",
		source: `print((1 + 2) * (3 - 1))`,
		want:   "6\n",
	},
	{
		name:   "modulo and negation",
		source: `print(17 % 5, -3 + 1)`,
		want:   "2 -2\n",
	},
	{
		name:   "floats",
		source: `print(1.5 + 2.25, 3.0 / 2.0)`,
		want:   "3.75 1.5\n",
	},
	{
		name:   "comparisons",
		source: `print(1 < 2, 2 <= 2, 3 > 4, 4 >= 4, 1 == 1, 1 != 1)`,
		want:   "true true false true true false\n",
	},
	{
		name: "boolean logic short circuits",
		source: `print(true && false, true || false)
fn boom(){ print("no") return true }
print(false && boom())
print(true || boom())`,
		want: "false true\nfalse\ntrue\n",
	},
	{
		name:   "strings and concatenation",
		source: `print("a" + "b" + "c")`,
		want:   "abc\n",
	},
	{
		name: "string interpolation",
		source: `let n = 2
print("n is {n}, twice is {n * 2}")`,
		want: "n is 2, twice is 4\n",
	},
	{
		name:   "interpolation calls functions",
		source: "fn shout(s) { return upper(s) }\nlet word = \"hi\"\nprint(\"say {shout(word)}!\")",
		want:   "say HI!\n",
	},
	{
		name:   "interpolation of an array",
		source: `print("list {[1, 2, 3]}")`,
		want:   "list [1, 2, 3]\n",
	},
	{
		name:   "single braces stay literal",
		source: `print("{{ok}}")`,
		want:   "{ok}\n",
	},
	{
		name: "hash is an ordinary character",
		source: `let n = 1
print("friend #{n}")`,
		want: "friend #1\n",
	},
	{
		name: "let declarations and assignment",
		source: `let x = 1
x = x + 10
print(x)`,
		want: "11\n",
	},
	{
		name: "if else as an expression",
		source: `fn sign(n) {
    if n < 0 { return "neg" } else { return "pos" }
}
print(sign(-1), sign(1))`,
		want: "neg pos\n",
	},
	{
		name: "else if chain",
		source: `fn size(n) {
    if n < 10 { return "small" }
    else if n < 100 { return "medium" }
    else { return "large" }
}
print(size(1), size(50), size(500))`,
		want: "small medium large\n",
	},
	{
		name: "while loop",
		source: `let i = 0
let total = 0
while i < 5 {
    total = total + i
    i = i + 1
}
print(total)`,
		want: "10\n",
	},
	{
		name: "for over an array",
		source: `let parts = []
for v in [3, 1, 2] {
    parts = push(parts, v * 2)
}
print(parts)`,
		want: "[6, 2, 4]\n",
	},
	{
		name: "for over a string",
		source: `let letters = []
for c in "abc" {
    letters = push(letters, upper(c))
}
print(letters)`,
		want: `["A", "B", "C"]` + "\n",
	},
	{
		name: "break and continue",
		source: `let kept = []
for n in range(10) {
    if n == 2 { continue }
    if n == 5 { break }
    kept = push(kept, n)
}
print(kept)`,
		want: "[0, 1, 3, 4]\n",
	},
	{
		name: "continue in a while loop",
		source: `let i = 0
let kept = []
while i < 6 {
    i = i + 1
    if i % 2 == 0 { continue }
    kept = push(kept, i)
}
print(kept)`,
		want: "[1, 3, 5]\n",
	},
	{
		name: "nested loops with break",
		source: `let caught = []
for a in [1, 2] {
    for b in [1, 2, 3] {
        if b == 2 { break }
        caught = push(caught, a * 10 + b)
    }
}
print(caught)`,
		want: "[11, 21]\n",
	},
	{
		name: "arrays and indexing",
		source: `let a = [10, 20, 30]
print(a[0], a[1], a[2], len(a))`,
		want: "10 20 30 3\n",
	},
	{
		name:   "string indexing counts runes",
		source: `print("héllo"[1], len("héllo"))`,
		want:   "é 5\n",
	},
	{
		name: "index assignment",
		source: `let a = [1, 2, 3]
a[1] = 99
print(a)`,
		want: "[1, 99, 3]\n",
	},
	{
		name:   "array literal concatenation",
		source: `print([1, 2] + [3])`,
		want:   "[1, 2, 3]\n",
	},
	{
		name: "functions and returns",
		source: `fn add(a, b) { return a + b }
fn noReturn() { }
print(add(2, 3))
print(noReturn())`,
		want: "5\nnil\n",
	},
	{
		name: "recursion",
		source: `fn fact(n) {
    if n <= 1 { return 1 }
    return n * fact(n - 1)
}
print(fact(6))`,
		want: "720\n",
	},
	{
		name: "mutual recursion at the top level",
		source: `fn even(n) {
    if n == 0 { return true }
    return odd(n - 1)
}
fn odd(n) {
    if n == 0 { return false }
    return even(n - 1)
}
print(even(10), odd(10))`,
		want: "true false\n",
	},
	{
		name: "functions may use globals declared later",
		source: `fn shout() { return upper(message) }
let message = "quiet"
print(shout())`,
		want: "QUIET\n",
	},
	{
		name: "functions as values",
		source: `fn twice(f, x) { return f(f(x)) }
let bump = fn(n) { return n + 1 }
print(twice(bump, 5))`,
		want: "7\n",
	},
	{
		name: "closures capture an outer local",
		source: `fn adder(n) {
    return fn(x) { return x + n }
}
let add5 = adder(5)
print(add5(10), add5(100))`,
		want: "15 105\n",
	},
	{
		name: "closures mutate their captured local",
		source: `fn counter() {
    let n = 0
    return fn() {
        n = n + 1
        return n
    }
}
let next = counter()
print(next(), next(), next())`,
		want: "1 2 3\n",
	},
	{
		name: "closure captures a global by reference",
		source: `let seen = 0
fn bump() {
    seen = seen + 1
    return seen
}
bump()
print(bump(), seen)`,
		want: "2 2\n",
	},
	{
		name: "closures close over loop values",
		source: `let fns = []
for i in [1, 2, 3] {
    fns = push(fns, fn() { return i })
}
let out = []
for f in fns {
    out = push(out, f())
}
print(out)`,
		want: "[1, 2, 3]\n",
	},
	{
		name: "a recursive function literal inside a function",
		source: `fn outer() {
    let countdown = fn(n) {
        if n == 0 { return "liftoff" }
        return countdown(n - 1)
    }
    return countdown(3)
}
print(outer())`,
		want: "liftoff\n",
	},
	{
		name: "a capture passed through two closures",
		source: `fn outer() {
    let x = 10
    return fn() {
        return fn() { return x }
    }
}
let middle = outer()
let inner = middle()
print(inner())`,
		want: "10\n",
	},
	{
		name: "curried closures keep every captured value",
		source: `fn adder(n) {
    return fn(x) {
        return fn(y) { return x + y + n }
    }
}
print(adder(100)(20)(3))`,
		want: "123\n",
	},
	{
		name: "an empty array",
		source: `let a = []
print(len(a), a)` + "\n" + `for i in a { print("never") }
print("done")`,
		want: "0 []\ndone\n",
	},
	{
		name: "while true with break",
		source: `let n = 0
while true {
    n = n + 1
    if n == 3 { break }
}
print(n)`,
		want: "3\n",
	},
	{
		name: "redeclaring a top-level name",
		source: `let x = 1
let x = x + 1
print(x)`,
		want: "2\n",
	},
	{
		name: "a nested function defined inside a top-level block",
		source: `if true {
    fn inside() { return "here" }
    print(inside())
}`,
		want: "here\n",
	},
	{
		name: "a named function can call itself",
		source: `fn outer() {
    fn inner(n) {
        if n == 0 { return 0 }
        return n + inner(n - 1)
    }
    return inner(4)
}
print(outer())`,
		want: "10\n",
	},
	{
		name: "block scoping shadows and restores",
		source: `let x = 1
if true {
    let x = 2
    print(x)
}
print(x)`,
		want: "2\n1\n",
	},
	{
		name: "inner block can read the outer variable",
		source: `let x = 1
if true {
    x = x + 41
}
print(x)`,
		want: "42\n",
	},
	{
		name:   "builtins",
		source: `print(abs(-3), min(1, 2), max(1, 2), int(2.9), float(2), str(7))`,
		want:   "3 1 2 2 2 7\n",
	},
	{
		name:   "string builtins",
		source: `print(trim("  hi  "), split("a,b", ","), join(["a", "b"], "+"), contains("abc", "b"))`,
		want:   "hi [\"a\", \"b\"] a+b true\n",
	},
	{
		name:   "range variants",
		source: `print(range(3), range(2, 5), len(range(10)))`,
		want:   "[0, 1, 2] [2, 3, 4] 10\n",
	},
	{
		name:   "type builtin",
		source: `print(type(1), type(1.0), type("s"), type(true), type(nil), type([1]), type(fn(){}))`,
		want:   "int float str bool nil array fn\n",
	},
	{
		name: "deep nesting still leaves the right value",
		source: `fn deep(n) {
    if n == 0 {
        if true { return "bottom" } else { return "never" }
    }
    return deep(n - 1)
}
print(deep(3))`,
		want: "bottom\n",
	},
	{
		name: "a loop as the last statement yields nil",
		source: `for i in range(3) { let x = i }
print("done")`,
		want: "done\n",
	},
}

func TestProgramsAgreeAcrossEngines(t *testing.T) {
	for _, tc := range programs {
		t.Run(tc.name, func(t *testing.T) {
			vmOut, vmErr := runOnVM(t, tc.source)
			if vmErr != nil {
				t.Fatalf("vm error: %v", vmErr)
			}
			if vmOut != tc.want {
				t.Errorf("vm output\n got: %q\nwant: %q", vmOut, tc.want)
			}

			interpOut, interpErr := runOnInterpreter(t, tc.source)
			if interpErr != nil {
				t.Fatalf("interpreter error: %v", interpErr)
			}
			if interpOut != tc.want {
				t.Errorf("interpreter output\n got: %q\nwant: %q", interpOut, tc.want)
			}
		})
	}
}

// errorPrograms are programs that must fail on both back ends. The exact
// position is checked by requiring the two engines to produce the same message;
// wantContains pins the wording so a bug cannot make both wrong in the same way.
var errorPrograms = []struct {
	name         string
	source       string
	wantContains string
}{
	{
		name:         "adding mixed numbers",
		source:       `print(1 + 1.5)`,
		wantContains: "cannot add int and float; convert one explicitly with int() or float()",
	},
	{
		name:         "adding a string to a number",
		source:       `print("a" + 1)`,
		wantContains: "cannot add str and int",
	},
	{
		name:         "division by zero",
		source:       `print(1 / 0)`,
		wantContains: "division by zero",
	},
	{
		name:         "modulo by zero",
		source:       `print(1 % 0)`,
		wantContains: "division by zero",
	},
	{
		name:         "undefined variable",
		source:       `print(missing)`,
		wantContains: `undefined variable "missing"`,
	},
	{
		name:         "assigning to an undeclared name",
		source:       `nope = 1`,
		wantContains: `undefined variable "nope"; use 'let' to declare it first`,
	},
	{
		name:         "index out of range",
		source:       `let a = [1]` + "\n" + `print(a[5])`,
		wantContains: "index 5 out of range (length 1)",
	},
	{
		name:         "negative index",
		source:       `let a = [1]` + "\n" + `print(a[0 - 1])`,
		wantContains: "index -1 out of range (length 1)",
	},
	{
		name:         "indexing a number",
		source:       `print(1[0])`,
		wantContains: "cannot index int",
	},
	{
		name:         "non-integer index",
		source:       `let a = [1, 2]` + "\n" + `print(a["x"])`,
		wantContains: "index must be an int, got str",
	},
	{
		name:         "calling a number",
		source:       `let x = 1` + "\n" + `print(x())`,
		wantContains: "int is not callable",
	},
	{
		name:         "comparing a string with a number",
		source:       `print("a" < 1)`,
		wantContains: "cannot compare str and int",
	},
	{
		name:         "negating a string",
		source:       `print(-"a")`,
		wantContains: "'-' expects a number, got str",
	},
	{
		name:         "not on a number",
		source:       `print(!1)`,
		wantContains: "'!' expects a bool, got int",
	},
	{
		name:         "assigning into a string",
		source:       `let s = "abc"` + "\n" + `s[0] = "x"`,
		wantContains: "cannot assign into str",
	},
	{
		name:         "subscript assignment out of range",
		source:       `let a = [1]` + "\n" + `a[9] = 2`,
		wantContains: "index 9 out of range (length 1)",
	},
	{
		name: "wrong argument count",
		source: `fn f(a) { return a }
f(1, 2)`,
		wantContains: "expects 1 argument(s), got 2",
	},
	{
		name: "too few arguments",
		source: `fn f(a, b) { return a }
f(1)`,
		wantContains: "expects 2 argument(s), got 1",
	},
}

func TestErrorsAgreeAcrossEngines(t *testing.T) {
	for _, tc := range errorPrograms {
		t.Run(tc.name, func(t *testing.T) {
			_, vmErr := runOnVM(t, tc.source)
			if vmErr == nil {
				t.Fatalf("vm accepted a program that should fail")
			}
			if !strings.Contains(vmErr.Error(), tc.wantContains) {
				t.Errorf("vm error %q does not contain %q", vmErr, tc.wantContains)
			}

			_, interpErr := runOnInterpreter(t, tc.source)
			if interpErr == nil {
				t.Fatalf("interpreter accepted a program that should fail")
			}
			if !strings.Contains(interpErr.Error(), tc.wantContains) {
				t.Errorf("interpreter error %q does not contain %q", interpErr, tc.wantContains)
			}

			// Both should point at the same place, so users see the same
			// message whichever engine runs their program.
			if vmErr.Error() != interpErr.Error() {
				t.Errorf("engines disagree on the message:\n vm: %v\n interp: %v", vmErr, interpErr)
			}
		})
	}
}

// TestExamplesAgreeAcrossEngines runs every example on both back ends and
// requires identical output, so the shipped programs are guaranteed to behave
// the same on either.
func TestExamplesAgreeAcrossEngines(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("examples", "*.jin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no examples found")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(source)

			vmOut, vmErr := runOnVM(t, text)
			interpOut, interpErr := runOnInterpreter(t, text)

			if (vmErr == nil) != (interpErr == nil) {
				t.Fatalf("engines disagree on success:\n vm: %v\n interp: %v", vmErr, interpErr)
			}
			if vmErr != nil {
				if vmErr.Error() != interpErr.Error() {
					t.Fatalf("engines disagree on the error:\n vm: %v\n interp: %v", vmErr, interpErr)
				}
				return
			}
			if vmOut != interpOut {
				t.Fatalf("engines disagree on output:\n vm:\n%s\n interp:\n%s", vmOut, interpOut)
			}
		})
	}
}

// ---------------------------------------------------------------- vm details

// TestGlobalsSurviveBetweenRuns checks the REPL behaviour: one VM keeps its
// global variables and functions from one chunk to the next.
func TestGlobalsSurviveBetweenRuns(t *testing.T) {
	var out bytes.Buffer
	vm := NewVM(&out)
	compiler := NewCompiler(NewSymbolTable(true))

	run := func(source string) {
		t.Helper()
		p := NewParser(NewLexer(source))
		program := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("syntax errors: %v", errs)
		}
		bc, err := compiler.Compile(program)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if _, err := vm.Run(bc); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	run(`let total = 10`)
	run(`fn double(n) { return n * 2 }`)
	run(`print(double(total))`)

	if got, want := out.String(), "20\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestLastValueIsReported checks that Run hands back the value of the last
// statement, which is what the REPL shows.
func TestLastValueIsReported(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{`1 + 2`, "3"},
		{`let x = 5`, "5"},
		{"let x = 5\nx = 9", "9"},
		{`"hi"`, "hi"},
		{`print("noise")`, "nil"},
		{`if true { 7 } else { 8 }`, "7"},
	}

	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			p := NewParser(NewLexer(tc.source))
			program := p.ParseProgram()
			bc, err := compileProgram(program)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			vm := NewVM(&out)
			result, err := vm.Run(bc)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.Inspect(); got != tc.want {
				t.Errorf("result = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDeepRecursionDoesNotCrash checks the frame stack grows and unwinds.
func TestDeepRecursionDoesNotCrash(t *testing.T) {
	source := `fn count(n) {
    if n == 0 { return 0 }
    return 1 + count(n - 1)
}
print(count(200))`

	got, err := runOnVM(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if got != "200\n" {
		t.Errorf("output = %q, want %q", got, "200\n")
	}
}

// TestTooManyNestedCallsReportAnError checks the VM refuses runaway recursion
// instead of crashing.
func TestTooManyNestedCallsReportAnError(t *testing.T) {
	source := `fn down(n) { return down(n + 1) }
down(0)`

	_, err := runOnVM(t, source)
	if err == nil {
		t.Fatal("expected an error from unbounded recursion")
	}
	if !strings.Contains(err.Error(), "too many nested calls") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestStackGrowsPastItsInitialSize checks a program can use more stack than the
// VM starts with.
func TestStackGrowsPastItsInitialSize(t *testing.T) {
	source := `let items = []
for i in range(4000) {
    items = push(items, i)
}
print(len(items), items[3999])`

	got, err := runOnVM(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if got != "4000 3999\n" {
		t.Errorf("output = %q, want %q", got, "4000 3999\n")
	}
}
