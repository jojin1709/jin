package main

import (
	"strings"
	"testing"
)

// parse is a test helper: it parses source and fails on any parse error.
func parse(t *testing.T, src string) *Program {
	t.Helper()
	p := NewParser(NewLexer(src))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %s", strings.Join(p.Errors(), "; "))
	}
	return program
}

func TestParseLet(t *testing.T) {
	program := parse(t, `let x: int = 5`)

	if len(program.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*LetStatement)
	if !ok {
		t.Fatalf("want LetStatement, got %T", program.Statements[0])
	}
	if stmt.Name.Value != "x" {
		t.Errorf("name: want x, got %s", stmt.Name.Value)
	}
	if stmt.Type == nil || stmt.Type.Name != "int" {
		t.Errorf("type annotation missing or wrong: %+v", stmt.Type)
	}
	if stmt.Value.String() != "5" {
		t.Errorf("value: want 5, got %s", stmt.Value.String())
	}
}

func TestParseFunction(t *testing.T) {
	program := parse(t, `fn add(a: int, b: int) -> int { return a + b }`)

	fn, ok := program.Statements[0].(*FunctionStatement)
	if !ok {
		t.Fatalf("want FunctionStatement, got %T", program.Statements[0])
	}
	if fn.Name.Value != "add" {
		t.Errorf("name: want add, got %s", fn.Name.Value)
	}
	if len(fn.Parameters) != 2 {
		t.Fatalf("want 2 params, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Name.Value != "a" || fn.Parameters[0].Type.Name != "int" {
		t.Errorf("first param wrong: %s", fn.Parameters[0].String())
	}
	if fn.ReturnType == nil || fn.ReturnType.Name != "int" {
		t.Errorf("return type wrong")
	}
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("want 1 body statement, got %d", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[0].(*ReturnStatement); !ok {
		t.Errorf("want ReturnStatement, got %T", fn.Body.Statements[0])
	}
}

func TestOperatorPrecedence(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"1 + 2 * 3", "(1 + (2 * 3))"},
		{"1 * 2 + 3", "((1 * 2) + 3)"},
		{"(1 + 2) * 3", "((1 + 2) * 3)"},
		{"a == b || c != d", "((a == b) || (c != d))"},
		{"!x && y", "((!x) && y)"},
		{"-a + b", "((-a) + b)"},
		{"a + b < c * d", "((a + b) < (c * d))"},
		{"a = b = c", "(a = (b = c))"},
		{"x >= 5 && y != 2 || x <= 1", "(((x >= 5) && (y != 2)) || (x <= 1))"},
	}

	for _, c := range cases {
		program := parse(t, c.src)
		stmt := program.Statements[0].(*ExpressionStatement)
		if got := stmt.Expression.String(); got != c.want {
			t.Errorf("%q\n want %s\n got  %s", c.src, c.want, got)
		}
	}
}

func TestParseCallsIndexAndMembers(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"add(1, 2)", "add(1, 2)"},
		{"print(greet(\"hi\"))", `print(greet("hi"))`},
		{"list[0]", "(list[0])"},
		{"obj.name", "obj.name"},
		{"a.b.c", "a.b.c"},
		{"getItems()[0]", "(getItems()[0])"},
	}

	for _, c := range cases {
		program := parse(t, c.src)
		stmt := program.Statements[0].(*ExpressionStatement)
		if got := stmt.Expression.String(); got != c.want {
			t.Errorf("%q\n want %s\n got  %s", c.src, c.want, got)
		}
	}
}

func TestParseControlFlow(t *testing.T) {
	src := `
for n in [1, 2, 3] {
    if n % 2 == 0 {
        print(n)
    } else if n == 3 {
        print("three")
    } else {
        print("odd")
    }
}
while x < 10 {
    x = x + 1
    if x == 5 { break }
}
`
	program := parse(t, src)

	if len(program.Statements) != 2 {
		t.Fatalf("want 2 top-level statements, got %d", len(program.Statements))
	}

	loop, ok := program.Statements[0].(*ForStatement)
	if !ok {
		t.Fatalf("want ForStatement, got %T", program.Statements[0])
	}
	if loop.VarName.Value != "n" {
		t.Errorf("loop variable: want n, got %s", loop.VarName.Value)
	}
	if len(loop.Body.Statements) != 1 {
		t.Fatalf("loop body: want 1 statement, got %d", len(loop.Body.Statements))
	}

	ifStmt, ok := loop.Body.Statements[0].(*IfStatement)
	if !ok {
		t.Fatalf("want IfStatement, got %T", loop.Body.Statements[0])
	}
	if _, ok := ifStmt.Alternative.(*IfStatement); !ok {
		t.Errorf("else-if should chain into an IfStatement, got %T", ifStmt.Alternative)
	}

	if _, ok := program.Statements[1].(*WhileStatement); !ok {
		t.Errorf("want WhileStatement, got %T", program.Statements[1])
	}
}

func TestParseArrayAndAnonymousFn(t *testing.T) {
	program := parse(t, `let f = fn(a) { return a * 2 }`)
	stmt := program.Statements[0].(*LetStatement)

	if _, ok := stmt.Value.(*FunctionLiteral); !ok {
		t.Fatalf("want FunctionLiteral, got %T", stmt.Value)
	}

	program = parse(t, `let xs = [1, 2, 3]`)
	stmt = program.Statements[0].(*LetStatement)
	arr, ok := stmt.Value.(*ArrayLiteral)
	if !ok {
		t.Fatalf("want ArrayLiteral, got %T", stmt.Value)
	}
	if len(arr.Elements) != 3 {
		t.Errorf("want 3 elements, got %d", len(arr.Elements))
	}
}

func TestParseMapLiterals(t *testing.T) {
	program := parse(t, `let m = {"a": 1, "b": 2}`)
	stmt := program.Statements[0].(*LetStatement)

	m, ok := stmt.Value.(*MapLiteral)
	if !ok {
		t.Fatalf("want MapLiteral, got %T", stmt.Value)
	}
	if len(m.Pairs) != 2 {
		t.Fatalf("want 2 pairs, got %d", len(m.Pairs))
	}
	if m.Pairs[0].Key.(*StringLiteral).Value != "a" {
		t.Errorf("first key is %s, want \"a\"", m.Pairs[0].Key.String())
	}
	if m.Pairs[1].Value.(*IntegerLiteral).Value != 2 {
		t.Errorf("second value is %s, want 2", m.Pairs[1].Value.String())
	}
	if m.String() != `{"a": 1, "b": 2}` {
		t.Errorf("String() is %s", m.String())
	}

	// An empty map is allowed, and keys are not limited to strings.
	empty := parse(t, `let e = {}`).Statements[0].(*LetStatement)
	if em, ok := empty.Value.(*MapLiteral); !ok || len(em.Pairs) != 0 {
		t.Errorf("want an empty MapLiteral, got %s", empty.Value.String())
	}

	mixed := parse(t, `let e = {1: "one", true: 2}`).Statements[0].(*LetStatement)
	if mm, ok := mixed.Value.(*MapLiteral); !ok || len(mm.Pairs) != 2 {
		t.Errorf("want a 2 pair MapLiteral, got %s", mixed.Value.String())
	}
}

func TestMapLiteralNeedsAValue(t *testing.T) {
	p := NewParser(NewLexer(`let m = {"a": }`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected a parse error for a map entry with no value")
	}
}

func TestParseErrorsAreReported(t *testing.T) {
	p := NewParser(NewLexer(`let = 5`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected a parse error for 'let = 5'")
	}
}

// A ';' separates statements wherever it appears, including after a statement
// that ends in a brace, which is what lets a whole nested definition sit on one
// line.
func TestSemicolonsSeparateStatements(t *testing.T) {
	program := parse(t, `fn outer() {
    fn a(n) { if n <= 0 { return "a" }; return b(n - 1) }
    fn b(n) { if n <= 0 { return "b" }; return a(n - 1) }; return a(5)
}`)

	if len(program.Statements) != 1 {
		t.Fatalf("want 1 top-level statement, got %d", len(program.Statements))
	}
	outer, ok := program.Statements[0].(*FunctionStatement)
	if !ok {
		t.Fatalf("want FunctionStatement, got %T", program.Statements[0])
	}
	// The body holds the two nested functions and the final return.
	if got := len(outer.Body.Statements); got != 3 {
		t.Fatalf("outer body: want 3 statements, got %d", got)
	}
	a, ok := outer.Body.Statements[0].(*FunctionStatement)
	if !ok {
		t.Fatalf("want FunctionStatement, got %T", outer.Body.Statements[0])
	}
	// The ';' inside a's body separates its if from its return.
	if got := len(a.Body.Statements); got != 2 {
		t.Errorf("a body: want 2 statements, got %d", got)
	}
	if _, ok := outer.Body.Statements[2].(*ReturnStatement); !ok {
		t.Errorf("want ReturnStatement last, got %T", outer.Body.Statements[2])
	}

	// A semicolon on its own is a separator, not a statement.
	loose := parse(t, "let a = 1;;\n;if true { print(a) };")
	if len(loose.Statements) != 2 {
		t.Errorf("want 2 statements, got %d", len(loose.Statements))
	}
}

func TestParseEmptyProgram(t *testing.T) {
	program := parse(t, "")
	if len(program.Statements) != 0 {
		t.Errorf("want 0 statements, got %d", len(program.Statements))
	}
}

func TestParseStruct(t *testing.T) {
	src := `struct Point {
    x: int,
    y: int
}
struct Empty {}`
	program := parse(t, src)
	if len(program.Statements) != 2 {
		t.Fatalf("want 2 statements, got %d", len(program.Statements))
	}

	st, ok := program.Statements[0].(*StructStatement)
	if !ok {
		t.Fatalf("want StructStatement, got %T", program.Statements[0])
	}
	if st.Name.Value != "Point" {
		t.Errorf("want name Point, got %q", st.Name.Value)
	}
	if len(st.Fields) != 2 {
		t.Fatalf("want 2 fields, got %d", len(st.Fields))
	}
	if st.Fields[0].Name.Value != "x" || st.Fields[0].Type.Name != "int" {
		t.Errorf("field 0 mismatch: %s: %s", st.Fields[0].Name.Value, st.Fields[0].Type.Name)
	}
	if st.Fields[1].Name.Value != "y" || st.Fields[1].Type.Name != "int" {
		t.Errorf("field 1 mismatch: %s: %s", st.Fields[1].Name.Value, st.Fields[1].Type.Name)
	}

	empty, ok := program.Statements[1].(*StructStatement)
	if !ok {
		t.Fatalf("want StructStatement, got %T", program.Statements[1])
	}
	if empty.Name.Value != "Empty" || len(empty.Fields) != 0 {
		t.Errorf("empty struct mismatch: name=%s fields=%d", empty.Name.Value, len(empty.Fields))
	}
}

func TestParseStructInsideBlockRejected(t *testing.T) {
	p := NewParser(NewLexer(`fn f() { struct Bad { x: int } }`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected parse error for struct inside a block, got none")
	}
	if !strings.Contains(p.Errors()[0], "only allowed at the top level") {
		t.Errorf("unexpected error message: %s", p.Errors()[0])
	}
}

func TestParseMethod(t *testing.T) {
	src := `fn (p: Point) length() -> int { return p.x }`
	program := parse(t, src)
	if len(program.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(program.Statements))
	}
	fn, ok := program.Statements[0].(*FunctionStatement)
	if !ok {
		t.Fatalf("want FunctionStatement, got %T", program.Statements[0])
	}
	if fn.Receiver == nil {
		t.Fatal("expected non-nil Receiver")
	}
	if fn.Receiver.Name.Value != "p" || fn.Receiver.Type.Name != "Point" {
		t.Errorf("receiver mismatch: %+v", fn.Receiver)
	}
	if fn.Name.Value != "length" {
		t.Errorf("name: want length, got %s", fn.Name.Value)
	}
	if fn.ReturnType == nil || fn.ReturnType.Name != "int" {
		t.Errorf("return type mismatch: %+v", fn.ReturnType)
	}
}

func TestParseMethodInsideBlockRejected(t *testing.T) {
	p := NewParser(NewLexer(`fn f() { fn (p: Point) bad() {} }`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected parse error for method inside block, got none")
	}
	if !strings.Contains(p.Errors()[0], "methods are only allowed at the top level") {
		t.Errorf("unexpected error message: %s", p.Errors()[0])
	}
}

func TestParseTryCatch(t *testing.T) {
	src := `try {
    let x = 1
} catch err {
    print(err)
}`
	program := parse(t, src)
	if len(program.Statements) != 1 {
		t.Fatalf("want 1 statement, got %d", len(program.Statements))
	}
	ts, ok := program.Statements[0].(*TryStatement)
	if !ok {
		t.Fatalf("want TryStatement, got %T", program.Statements[0])
	}
	if len(ts.TryBody.Statements) != 1 {
		t.Fatalf("want 1 try statement, got %d", len(ts.TryBody.Statements))
	}
	if ts.CatchIdent.Value != "err" {
		t.Errorf("want catch ident err, got %s", ts.CatchIdent.Value)
	}
	if len(ts.CatchBody.Statements) != 1 {
		t.Fatalf("want 1 catch statement, got %d", len(ts.CatchBody.Statements))
	}
}
