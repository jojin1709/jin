package main

import (
	"bytes"
	"strings"
	"testing"
)

func evalSrc(t *testing.T, src string, out *bytes.Buffer) (Object, error) {
	t.Helper()
	p := NewParser(NewLexer(src))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors for %q: %s", src, strings.Join(p.Errors(), "; "))
	}
	return NewInterpreter(out).Run(program, NewEnvironment())
}

func run(t *testing.T, src string) Object {
	t.Helper()
	obj, err := evalSrc(t, src, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runtime error for %q: %v", src, err)
	}
	return obj
}

func runInt(t *testing.T, src string) int64 {
	t.Helper()
	obj := run(t, src)
	n, ok := obj.(*Integer)
	if !ok {
		t.Fatalf("%q: want int, got %s (%s)", src, obj.Type(), obj.Inspect())
	}
	return n.Value
}

func runStr(t *testing.T, src string) string {
	t.Helper()
	obj := run(t, src)
	s, ok := obj.(*String)
	if !ok {
		t.Fatalf("%q: want str, got %s (%s)", src, obj.Type(), obj.Inspect())
	}
	return s.Value
}

func runBool(t *testing.T, src string) bool {
	t.Helper()
	obj := run(t, src)
	b, ok := obj.(*Boolean)
	if !ok {
		t.Fatalf("%q: want bool, got %s (%s)", src, obj.Type(), obj.Inspect())
	}
	return b.Value
}

// ---------------------------------------------------------------- tests

func TestIntegerArithmetic(t *testing.T) {
	cases := []struct {
		src  string
		want int64
	}{
		{"5", 5},
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"7 / 2", 3}, // integer division
		{"7 % 3", 1},
		{"-5 + 10", 5},
		{"2 * 3 - 4 / 2", 4},
	}
	for _, c := range cases {
		if got := runInt(t, c.src); got != c.want {
			t.Errorf("%q: want %d, got %d", c.src, c.want, got)
		}
	}
}

func TestBooleanLogic(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"true", true},
		{"!false", true},
		{"1 < 2", true},
		{"2 <= 2", true},
		{"3 > 4", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{`"a" == "a"`, true},
		{"true && true", true},
		{"true && false", false},
		{"false || true", true},
		{"nil == nil", true},
		{"[1, 2] == [1, 2]", true},
		{"[1, 2] == [1, 3]", false},
	}
	for _, c := range cases {
		if got := runBool(t, c.src); got != c.want {
			t.Errorf("%q: want %v, got %v", c.src, c.want, got)
		}
	}
}

func TestShortCircuitDoesNotEvaluateRightSide(t *testing.T) {
	// If && did not short-circuit, the undefined variable would raise.
	if got := runBool(t, `false && undefinedVar`); got != false {
		t.Errorf("want false, got %v", got)
	}
	if got := runBool(t, `true || undefinedVar`); got != true {
		t.Errorf("want true, got %v", got)
	}
}

func TestStrings(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`"a" + "b"`, "ab"},
		{"let name = \"Jin\"\n\"Hello, {name}!\"", "Hello, Jin!"},
		{`let n = 5
"{n} doubled is {n * 2}"`, "5 doubled is 10"},
		{`let n = 5
"friend #{n}"`, "friend #5"},
		{`"{{literal}}"`, "{literal}"},
		{`"len " + str(len("abc"))`, "len 3"},
	}
	for _, c := range cases {
		if got := runStr(t, c.src); got != c.want {
			t.Errorf("%q: want %q, got %q", c.src, c.want, got)
		}
	}
}

func TestVariablesAndAssignment(t *testing.T) {
	if got := runInt(t, "let x = 10\nx = x + 5\nx"); got != 15 {
		t.Errorf("want 15, got %d", got)
	}
	if got := runInt(t, "let a = 1\nlet b = 2\na + b"); got != 3 {
		t.Errorf("want 3, got %d", got)
	}
}

func TestIfElseChain(t *testing.T) {
	src := `
fn classify(n) {
    if n < 0 { return "negative" }
    else if n == 0 { return "zero" }
    else { return "positive" }
}
classify(-3) + "," + classify(0) + "," + classify(7)
`
	if got := runStr(t, src); got != "negative,zero,positive" {
		t.Errorf("got %q", got)
	}
}

func TestWhileLoopWithBreakAndContinue(t *testing.T) {
	src := `
let s = ""
let n = 0
while n < 10 {
    n = n + 1
    if n == 3 { continue }
    if n == 6 { break }
    s = s + str(n)
}
s
`
	if got := runStr(t, src); got != "1245" {
		t.Errorf("got %q", got)
	}
}

func TestForLoops(t *testing.T) {
	if got := runInt(t, "let t = 0\nfor x in [1, 2, 3, 4] { t = t + x }\nt"); got != 10 {
		t.Errorf("array loop: want 10, got %d", got)
	}
	if got := runStr(t, "let s = \"\"\nfor ch in \"abc\" { s = s + ch + \"-\" }\ns"); got != "a-b-c-" {
		t.Errorf("string loop: got %q", got)
	}
	if got := runInt(t, "let t = 0\nfor x in range(5) { t = t + x }\nt"); got != 10 {
		t.Errorf("range loop: want 10, got %d", got)
	}
}

func TestArrays(t *testing.T) {
	if got := runInt(t, "let a = [1, 2, 3]\na[0] = 99\na[0]"); got != 99 {
		t.Errorf("index assignment: want 99, got %d", got)
	}
	if got := runInt(t, "[10, 20, 30][2]"); got != 30 {
		t.Errorf("index read: want 30, got %d", got)
	}
	if got := runInt(t, "len([1, 2] + [3, 4])"); got != 4 {
		t.Errorf("array concat: want 4, got %d", got)
	}
	if got := runStr(t, `"abc"[1]`); got != "b" {
		t.Errorf("string index: want b, got %q", got)
	}
}

func TestFunctionsAndRecursion(t *testing.T) {
	src := `
fn fib(n) {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}
fib(15)
`
	if got := runInt(t, src); got != 610 {
		t.Errorf("fib(15): want 610, got %d", got)
	}

	// Implicit return of the last expression.
	if got := runInt(t, "fn double(x) { x * 2 }\ndouble(21)"); got != 42 {
		t.Errorf("implicit return: want 42, got %d", got)
	}
}

func TestClosures(t *testing.T) {
	src := `
fn counter() {
    let n = 0
    return fn() {
        n = n + 1
        return n
    }
}
let c = counter()
c()
c()
c()
`
	if got := runInt(t, src); got != 3 {
		t.Errorf("closure counter: want 3, got %d", got)
	}
}

func TestBuiltins(t *testing.T) {
	if got := runInt(t, "len(\"hello\")"); got != 5 {
		t.Errorf("len: want 5, got %d", got)
	}
	if got := runInt(t, "len(push([1, 2], 3))"); got != 3 {
		t.Errorf("push: want 3, got %d", got)
	}
	// push must not mutate the original array.
	if got := runInt(t, "let a = [1]\nlet b = push(a, 2)\nlen(a) + len(b)"); got != 3 {
		t.Errorf("push immutability: want 3, got %d", got)
	}
	if got := runStr(t, "upper(\"abc\")"); got != "ABC" {
		t.Errorf("upper: got %q", got)
	}
	if got := runStr(t, `join(split("a,b,c", ","), "-")`); got != "a-b-c" {
		t.Errorf("split/join: got %q", got)
	}
	if got := runBool(t, `contains("hello", "ell")`); got != true {
		t.Errorf("contains: want true")
	}
	if got := runStr(t, "type(1.5)"); got != "float" {
		t.Errorf("type: got %q", got)
	}
	if got := runInt(t, "int(3.9) + max(1, 2, 3)"); got != 6 {
		t.Errorf("int/max: want 6, got %d", got)
	}
	if got := runInt(t, "abs(-7)"); got != 7 {
		t.Errorf("abs: want 7, got %d", got)
	}
}

func TestPrintOutput(t *testing.T) {
	var out bytes.Buffer
	if _, err := evalSrc(t, `print("hello")`+"\n"+`print(1 + 2, "three")`, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != "hello\n3 three\n" {
		t.Errorf("want %q, got %q", "hello\n3 three\n", got)
	}
}

func TestRuntimeErrors(t *testing.T) {
	cases := []struct {
		src     string
		wantSub string
	}{
		{`1 + "x"`, "cannot add"},
		{"let n = 1\nn + 1.5", "convert one explicitly"},
		{"undefinedThing", "undefined variable"},
		{"1 / 0", "division by zero"},
		{"if 1 { }", "expected a bool"},
		{"fn f(a) { return a }\nf(1, 2)", "expects 1 argument"},
		{"break", "only allowed inside a loop"},
		{"return 1", "only allowed inside a function"},
		{"5[0]", "cannot index"},
		{"[1, 2][9]", "out of range"},
		{"[1, 2] - [1]", "cannot use -"},
		{"let a = 1\na.b", "cannot read a field of int"},
		{`"hi {1 +}"`, "bad interpolation"},
	}
	for _, c := range cases {
		_, err := evalSrc(t, c.src, &bytes.Buffer{})
		if err == nil {
			t.Errorf("%q: expected an error containing %q, got none", c.src, c.wantSub)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%q: error %q does not contain %q", c.src, err.Error(), c.wantSub)
		}
	}
}
