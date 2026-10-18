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

func TestParseErrorsAreReported(t *testing.T) {
	p := NewParser(NewLexer(`let = 5`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected a parse error for 'let = 5'")
	}
}

func TestParseEmptyProgram(t *testing.T) {
	program := parse(t, "")
	if len(program.Statements) != 0 {
		t.Errorf("want 0 statements, got %d", len(program.Statements))
	}
}
