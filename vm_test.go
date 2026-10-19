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
		name: "map literal and lookup",
		source: `let person = {"name": "Ada", "age": 36}
print(person)
print(person["name"], person["age"], len(person))`,
		want: "{\"name\": \"Ada\", \"age\": 36}\nAda 36 2\n",
	},
	{
		name: "empty map",
		source: `let m = {}
print(m, len(m), type(m))`,
		want: "{} 0 map\n",
	},
	{
		name: "map keys of every allowed kind",
		source: `let m = {1: "one", "two": 2, true: 3}
print(m[1], m["two"], m[true], len(m))`,
		want: "one 2 3 3\n",
	},
	{
		name: "map assignment adds and replaces",
		source: `let m = {"a": 1}
m["a"] = 2
m["b"] = 3
print(m, len(m))`,
		want: "{\"a\": 2, \"b\": 3} 2\n",
	},
	{
		name: "map keeps insertion order",
		source: `let m = {}
m["z"] = 1
m["a"] = 2
m["m"] = 3
print(m)
print(keys(m), values(m))`,
		want: "{\"z\": 1, \"a\": 2, \"m\": 3}\n[\"z\", \"a\", \"m\"] [1, 2, 3]\n",
	},
	{
		name: "has and get with a default",
		source: `let m = {"a": 1}
print(has(m, "a"), has(m, "b"))
print(get(m, "a", 0), get(m, "b", 0))
print(contains(m, "a"))`,
		want: "true false\n1 0\ntrue\n",
	},
	{
		name: "delete removes a key",
		source: `let m = {"a": 1, "b": 2, "c": 3}
delete(m, "b")
print(m, len(m), has(m, "b"))
delete(m, "nope")
print(m, len(m))`,
		want: "{\"a\": 1, \"c\": 3} 2 false\n{\"a\": 1, \"c\": 3} 2\n",
	},
	{
		name: "count words in a map",
		source: `let words = ["pear", "apple", "pear", "fig", "apple", "pear"]
let counts = {}
for w in words {
    counts[w] = get(counts, w, 0) + 1
}
print(counts)
for k in counts { print(k, counts[k]) }`,
		want: "{\"pear\": 3, \"apple\": 2, \"fig\": 1}\npear 3\napple 2\nfig 1\n",
	},
	{
		name: "nested maps and arrays",
		source: `let cfg = {"ports": [80, 443], "server": {"host": "local", "up": true}}
print(cfg["server"]["host"], cfg["server"]["up"], cfg["ports"][1])`,
		want: "local true 443\n",
	},
	{
		name: "maps inside functions and closures",
		source: `fn tally(items) {
    let m = {}
    for it in items { m[it] = get(m, it, 0) + 1 }
    return m
}
print(tally(["x", "y", "x"]))`,
		want: "{\"x\": 2, \"y\": 1}\n",
	},
	{
		name: "map of arrays built in a loop",
		source: `let groups = {}
for n in range(6) {
    let key = n % 2
    if has(groups, key) { groups[key] = push(groups[key], n) }
    else { groups[key] = [n] }
}
print(groups[0], groups[1])`,
		want: "[0, 2, 4] [1, 3, 5]\n",
	},
	{
		name: "sort reverse and slice",
		source: `print(sort([3, 1, 2]))
print(sort([2.5, 1.5, 2.0]))
print(sort(["pear", "apple", "fig"]))
print(reverse([1, 2, 3]), slice([1, 2, 3, 4], 1, 3))
print(reverse("abc"), slice("hello", 1, 4))`,
		want: "[1, 2, 3]\n[1.5, 2, 2.5]\n[\"apple\", \"fig\", \"pear\"]\n[3, 2, 1] [2, 3]\ncba ell\n",
	},
	{
		name: "sort leaves the original alone",
		source: `let a = [3, 1, 2]
let b = sort(a)
print(a, b)`,
		want: "[3, 1, 2] [1, 2, 3]\n",
	},
	{
		name:   "min and max over an array",
		source: `print(min([4, 2, 9]), max([4, 2, 9]), min(3, 1), max(1.5, 2))`,
		want:   "2 9 1 2\n",
	},
	{
		name: "string helpers",
		source: `print(replace("a-b-c", "-", "+"))
print(find("hello", "ll"), find("hello", "zz"))
print(starts_with("hello", "he"), ends_with("hello", "lo"))
print(repeat("ab", 3))
print(chars("hi"), lines("one\ntwo\n"))`,
		want: "a+b+c\n2 -1\ntrue true\nababab\n[\"h\", \"i\"] [\"one\", \"two\"]\n",
	},
	{
		name: "number helpers",
		source: `print(floor(2.7), ceil(2.1), round(2.5))
print(floor(7), ceil(7), round(7))
print(sqrt(16), pow(2, 10), pow(2, 0.5))`,
		want: "2 3 3\n7 7 7\n4 1024 1.4142135623730951\n",
	},
	{
		name: "map inside a closure captures the map",
		source: `fn counter() {
    let hits = {}
    return fn(name) {
        hits[name] = get(hits, name, 0) + 1
        return hits[name]
    }
}
let bump = counter()
print(bump("a"), bump("a"), bump("b"))`,
		want: "1 2 1\n",
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
	{
		name: "sibling closures share one variable",
		source: `fn make() {
    let n = 0
    let inc = fn() { n = n + 1 }
    let show = fn() { return n }
    inc()
    inc()
    return show()
}
print(make())`,
		want: "2\n",
	},
	{
		name: "two sibling closures write and read the same variable",
		source: `fn pair() {
    let n = 0
    let up = fn() {
        n = n + 1
        return n
    }
    let down = fn() {
        n = n - 1
        return n
    }
    return [up, down]
}
let p = pair()
print(p[0](), p[0](), p[1](), p[0]())`,
		want: "1 2 1 2\n",
	},
	{
		name: "a closure and its caller share the variable",
		source: `fn make() {
    let total = 0
    let add = fn(n) { total = total + n }
    let show = fn() { return total }
    add(3)
    add(4)
    return show()
}
print(make())`,
		want: "7\n",
	},
	{
		name: "a captured local still works in a loop",
		source: `fn make() {
    let n = 0
    let add = fn() { n = n + 1 }
    let show = fn() { return n }
    for i in range(3) { add() }
    return show()
}
print(make())`,
		want: "3\n",
	},
	{
		name: "closures in a loop share a variable from outside it",
		source: `fn make() {
    let n = 0
    let fns = []
    for i in range(3) {
        fns = push(fns, fn() { n = n + 1; return n })
    }
    for f in fns { f() }
    return n
}
print(make())`,
		want: "3\n",
	},
	{
		name: "two closures made together share their variable",
		source: `fn make() {
    let n = 5
    let pair = [fn() { n = n + 1; return n }, fn() { return n }]
    return pair[0]() + pair[1]()
}
print(make())`,
		want: "12\n",
	},
	{
		name: "a closure in a while body keeps each pass's value",
		source: `let fns = []
let i = 0
while i < 3 {
    let captured = i
    fns = push(fns, fn() { return captured })
    i = i + 1
}
let out = []
for f in fns { out = push(out, f()) }
print(out)`,
		want: "[0, 1, 2]\n",
	},
	{
		name: "loop variables captured by two nested loops",
		source: `let fns = []
for i in range(2) {
    for j in range(2) {
        fns = push(fns, fn() { return [i, j] })
    }
}
let out = []
for f in fns { out = push(out, f()) }
print(out)`,
		want: "[[0, 0], [0, 1], [1, 0], [1, 1]]\n",
	},
	{
		name: "a closure in a loop body keeps its own copy of the loop variable",
		source: `let fns = []
for i in range(3) {
    let box = fn() { return i * 10 }
    fns = push(fns, box)
}
let out = []
for f in fns { out = push(out, f()) }
print(out)`,
		want: "[0, 10, 20]\n",
	},
	{
		name: "mutual recursion between two functions in the same body",
		source: `fn outer() {
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
print(outer())`,
		want: "b\n",
	},
	{
		name: "a nested function can call one defined below it",
		source: `fn outer() {
    fn first() { return second() }
    fn second() { return "second" }
    return first()
}
print(outer())`,
		want: "second\n",
	},
	{
		name: "mutually recursive nested functions keep every call",
		source: `fn outer() {
    fn even(n) {
        if n == 0 { return true }
        return odd(n - 1)
    }
    fn odd(n) {
        if n == 0 { return false }
        return even(n - 1)
    }
    return [even(10), odd(10)]
}
print(outer())`,
		want: "[true, false]\n",
	},
	{
		name: "a capture shared two levels down",
		source: `fn outer() {
    let n = 0
    let mid = fn() {
        let bump = fn() { n = n + 1 }
        let show = fn() { return n }
        bump()
        bump()
        return show()
    }
    return mid()
}
print(outer())`,
		want: "2\n",
	},
	{
		name: "a returned pair keeps writing to one shared variable",
		source: `fn counter() {
    let n = 0
    let inc = fn() { n = n + 1 }
    let show = fn() { return n }
    return [inc, show]
}
let c = counter()
c[0]()
c[0]()
c[0]()
print(c[1]())`,
		want: "3\n",
	},
	{
		name: "sibling closures still share a global",
		source: `let hits = 0
let a = fn() { hits = hits + 1 }
let b = fn() { return hits }
a()
a()
print(b())`,
		want: "2\n",
	},
	{
		name: "a local can shadow a built-in name",
		source: `fn make() {
    let get = fn(x) { return x * 2 }
    return get(21)
}
print(make(), len([1, 2]))`,
		want: "42 2\n",
	},
	{
		name: "a closure made in an if branch keeps its value",
		source: `fn pick(flag) {
    let f = fn() { return "none" }
    if flag {
        let inner = "yes"
        f = fn() { return inner }
    } else {
        let other = "no"
        f = fn() { return other }
    }
    return f()
}
print(pick(true), pick(false))`,
		want: "yes no\n",
	},
	{
		name: "semicolons separate statements anywhere",
		source: `fn classify(n) {
    if n < 0 { return "neg" }; return "non-neg"
}
print(classify(-1), classify(1));
let total = 0;
for i in range(3) { total = total + i };;
print(total)`,
		want: "neg non-neg\n3\n",
	},
	{
		name: "each call to a factory gets its own cells",
		source: `fn counter() {
    let n = 0
    return fn() {
        n = n + 1
        return n
    }
}
let a = counter()
let b = counter()
print(a(), a(), b(), a(), b())`,
		want: "1 2 1 3 2\n",
	},
	{
		name: "a captured loop value survives the loop that made it",
		source: `fn make() {
    let kept = []
    for i in range(3) {
        kept = push(kept, fn() { return i })
    }
    let out = []
    for f in kept { out = push(out, f()) }
    return out
}
print(make(), make())`,
		want: "[0, 1, 2] [0, 1, 2]\n",
	},
	{
		name: "deeper recursion keeps every capture apart",
		source: `fn chain(n) {
    if n == 0 { return fn() { return 0 } }
    let depth = n
    let below = chain(n - 1)
    return fn() {
        let here = depth
        return here + below()
    }
}
print(chain(4)())`,
		want: "10\n",
	},
	{
		name: "break and continue leave the loop's cells alone",
		source: `let fns = []
for i in range(4) {
    if i % 2 == 1 { continue }
    let seen = i
    fns = push(fns, fn() { return seen })
}
let out = []
for f in fns { out = push(out, f()) }
print(out)

fn last_before_break() {
    let kept = fn() { return -1 }
    for i in range(5) {
        if i == 3 { break }
        kept = fn() { return i }
    }
    return kept()
}
print(last_before_break())

fn while_version() {
    let i = 0
    let fns = []
    while i < 4 {
        if i == 2 { i = i + 1; continue }
        let held = i * 10
        fns = push(fns, fn() { return held })
        i = i + 1
    }
    let out = []
    for f in fns { out = push(out, f()) }
    return out
}
print(while_version())`,
		want: "[0, 2]\n2\n[0, 10, 30]\n",
	},
	{
		name: "records: build, read and write a field",
		source: `struct Point { x: int, y: int }

let p = Point(1, 2)
print(p)
p.x = 10
print(p.x, p.y, p)

fn shift(pt: Point, d: int) -> Point {
    pt.x = pt.x + d
    pt.y = pt.y + d
    return pt
}
let moved = shift(p, 5)
print(moved, p, moved == p)`,
		want: "Point{x: 1, y: 2}\n10 2 Point{x: 10, y: 2}\nPoint{x: 15, y: 7} Point{x: 15, y: 7} true\n",
	},
	{
		name: "records: a record inside a record, an array and a map",
		source: `struct Point { x: int, y: int }
struct Line { a: Point, b: Point }

let l = Line(Point(0, 0), Point(3, 4))
print(l)
print(l.b.y, type(l), type(l.a))
print(l.a == Point(0, 0), l.a == Point(1, 0))

let pts = [Point(1, 1), Point(2, 2)]
print(pts)
print(len(pts), pts[1].y)
let named = {"origin": Point(0, 0)}
print(named["origin"], named["origin"].x)`,
		want: "Line{a: Point{x: 0, y: 0}, b: Point{x: 3, y: 4}}\n" +
			"4 Line Point\ntrue false\n" +
			"[Point{x: 1, y: 1}, Point{x: 2, y: 2}]\n2 2\n" +
			"Point{x: 0, y: 0} 0\n",
	},
	{
		name: "records: an assignment is an expression",
		source: `let a = [1, 2, 3]
print(1 + (a[0] = 5))
print(a)
struct Point { x: int, y: int }
let p = Point(1, 2)
print((p.x = 7) * 2, p)
print((p.y = 3) == 3, p)
let m = {"k": 0}
print((m["k"] = 9) + 1, m)`,
		want: "6\n[5, 2, 3]\n14 Point{x: 7, y: 2}\ntrue Point{x: 7, y: 3}\n10 {\"k\": 9}\n",
	},
	{
		name: "records: a field holding the same record again",
		source: `struct Node { value: int, next: Node }

let head = Node(1, Node(2, Node(3, nil)))
let n = head
let total = 0
while n != nil {
    total = total + n.value
    n = n.next
}
print(total)
print(head.next.next.value, type(head.next))
print(head)`,
		want: "6\n3 Node\n" +
			"Node{value: 1, next: Node{value: 2, next: Node{value: 3, next: nil}}}\n",
	},
	{
		name: "records: method call, receiver mutation, and return value",
		source: `struct Counter { count: int }

fn (c: Counter) add(n: int) -> int {
    c.count = c.count + n
    return c.count
}

fn (c: Counter) get_val() -> int {
    return c.count
}

let c = Counter(10)
print(c.add(5))
print(c.get_val(), c.count)
c.add(20)
print(c.get_val())`,
		want: "15\n15 15\n35\n",
	},
	{
		name: "records: method calling another method and bound method in variable",
		source: `struct Point { x: int, y: int }

fn (p: Point) sum() -> int {
    return p.x + p.y
}

fn (p: Point) scale_and_sum(factor: int) -> int {
    p.x = p.x * factor
    p.y = p.y * factor
    return p.sum()
}

let pt = Point(2, 3)
print(pt.sum())
print(pt.scale_and_sum(10))
print(pt)

let f = pt.sum
print(f())`,
		want: "5\n50\nPoint{x: 20, y: 30}\n50\n",
	},
	{
		name: "records: method defined before struct declaration",
		source: `fn (p: Point) describe() -> str {
    return "({p.x}, {p.y})"
}

struct Point { x: int, y: int }

let p = Point(7, 9)
print(p.describe())`,
		want: "(7, 9)\n",
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
	{
		name: "a missing map key",
		source: `let m = {"a": 1}
print(m["zz"])`,
		wantContains: `key "zz" is not in the map`,
	},
	{
		name: "int key is a key, not a position",
		source: `let m = {"a": 1, "b": 2}
print(m[0])`,
		wantContains: "key 0 is not in the map",
	},
	{
		name: "an array cannot be a map key",
		source: `let m = {}
m[[1]] = 2`,
		wantContains: "cannot use array as a map key; keys must be int, str or bool",
	},
	{
		name:         "sorting a mixed array",
		source:       `print(sort([1, "a"]))`,
		wantContains: "sort() needs a single kind of value",
	},
	{
		name:         "sorting something that is not an array",
		source:       `print(sort("abc"))`,
		wantContains: "sort() expects array, got str",
	},
	{
		name:         "slicing past the end",
		source:       `print(slice([1, 2], 0, 9))`,
		wantContains: "slice() end 9 is out of range",
	},
	{
		name:         "reversing a number",
		source:       `print(reverse(5))`,
		wantContains: "reverse() expects array or str, got int",
	},
	{
		name: "calling error",
		source: `print("before")
error("something broke")`,
		wantContains: "something broke",
	},
	{
		name:         "a random bound of zero",
		source:       `print(random(0))`,
		wantContains: "random() needs a bound above 0",
	},
	{
		name:         "looping over a map is fine but lopping over a number is not",
		source:       `for k in 5 { print(k) }`,
		wantContains: "cannot loop over int",
	},
}

// TestFileBuiltinsAgreeAcrossEngines uses a real temporary directory, so the
// two engines can be compared on something that actually touches the disk.
func TestFileBuiltinsAgreeAcrossEngines(t *testing.T) {
	// Jin string literals use \\ for a backslash, so a Windows path has to be
	// written with forward slashes or escaped.
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "notes.txt"))

	source := `let path = "` + path + `"
print(file_exists(path))
write_file(path, "first\nsecond\n")
print(file_exists(path))
print(read_file(path))
append_file(path, "third\n")
print(len(lines(read_file(path))))
print(lines(read_file(path)))
delete_file(path)
print(file_exists(path))`

	want := "false\ntrue\nfirst\nsecond\n\n3\n[\"first\", \"second\", \"third\"]\nfalse\n"

	vmOut, vmErr := runOnVM(t, source)
	if vmErr != nil {
		t.Fatalf("vm error: %v", vmErr)
	}
	if vmOut != want {
		t.Errorf("vm output\n got: %q\nwant: %q", vmOut, want)
	}

	// Put the file back so the interpreter walk starts from the same place.
	if _, err := os.Stat(path); err == nil {
		os.Remove(path)
	}
	interpOut, interpErr := runOnInterpreter(t, source)
	if interpErr != nil {
		t.Fatalf("interpreter error: %v", interpErr)
	}
	if interpOut != want {
		t.Errorf("interpreter output\n got: %q\nwant: %q", interpOut, want)
	}
}

func TestFileBuiltinErrors(t *testing.T) {
	missing := filepath.ToSlash(filepath.Join(t.TempDir(), "not-there.txt"))
	cases := []struct{ name, source, want string }{
		{
			name:   "reading a missing file",
			source: `print(read_file("` + missing + `"))`,
			want:   "read_file() could not read " + missing,
		},
		{
			name:   "creating a file below a missing directory",
			source: `write_file("` + missing + `/child.txt", "x")`,
			want:   "write_file() could not write",
		},
		{
			name:   "deleting a missing file",
			source: `delete_file("` + missing + `")`,
			want:   "delete_file() could not delete " + missing,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, vmErr := runOnVM(t, tc.source)
			if vmErr == nil {
				t.Fatalf("vm accepted %s", tc.source)
			}
			if !strings.Contains(vmErr.Error(), tc.want) {
				t.Errorf("vm error %q does not contain %q", vmErr, tc.want)
			}

			_, interpErr := runOnInterpreter(t, tc.source)
			if interpErr == nil {
				t.Fatalf("interpreter accepted %s", tc.source)
			}
			if !strings.Contains(interpErr.Error(), tc.want) {
				t.Errorf("interpreter error %q does not contain %q", interpErr, tc.want)
			}
		})
	}
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

// TestAStoppedLineDoesNotStrandACapturedValue checks the REPL case where a
// line captures a variable, then fails. The closure that escaped into a global
// has to keep the value it captured, not a stack slot the next line reuses.
func TestAStoppedLineDoesNotStrandACapturedValue(t *testing.T) {
	var out bytes.Buffer
	vm := NewVM(&out)
	compiler := NewCompiler(NewSymbolTable(true))

	run := func(source string) error {
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
		_, err = vm.Run(bc)
		return err
	}

	// The block captures a local, hands the closure to a global, then the same
	// line stops with an error, so the frame never returns.
	err := run(`let saved = nil
if true {
    let x = 41
    saved = fn() { return x + 1 }
}
error("stop here")`)
	if err == nil {
		t.Fatal("expected the line to fail")
	}

	if err := run(`print(saved())`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := out.String(), "42\n"; got != want {
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
