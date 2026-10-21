package main

import "testing"

type expected struct {
	Type    TokenType
	Literal string
}

func TestNextToken(t *testing.T) {
	input := `# a comment
let x = 5
let y = 3.14
let name = "Jin"
fn add(a, b) -> int {
    return a + b
}
if x >= 5 && y != 2 || x <= 1 {
    print("hi, {name}!")
}
`

	tests := []expected{
		{LET, "let"}, {IDENT, "x"}, {ASSIGN, "="}, {INT, "5"},
		{LET, "let"}, {IDENT, "y"}, {ASSIGN, "="}, {FLOAT, "3.14"},
		{LET, "let"}, {IDENT, "name"}, {ASSIGN, "="}, {STRING, "Jin"},
		{FUNCTION, "fn"}, {IDENT, "add"}, {LPAREN, "("}, {IDENT, "a"},
		{COMMA, ","}, {IDENT, "b"}, {RPAREN, ")"}, {ARROW, "->"},
		{IDENT, "int"}, {LBRACE, "{"},
		{RETURN, "return"}, {IDENT, "a"}, {PLUS, "+"}, {IDENT, "b"},
		{RBRACE, "}"},
		{IF, "if"}, {IDENT, "x"}, {GE, ">="}, {INT, "5"}, {AND, "&&"},
		{IDENT, "y"}, {NOT_EQ, "!="}, {INT, "2"}, {OR, "||"},
		{IDENT, "x"}, {LE, "<="}, {INT, "1"}, {LBRACE, "{"},
		{IDENT, "print"}, {LPAREN, "("}, {STRING, "hi, {name}!"}, {RPAREN, ")"},
		{RBRACE, "}"},
		{EOF, ""},
	}

	lex := NewLexer(input)

	for i, want := range tests {
		got := lex.NextToken()
		if got.Type != want.Type || got.Literal != want.Literal {
			t.Fatalf("tests[%d] wrong token.\n want=%s(%q)\n got =%s(%q)",
				i, want.Type, want.Literal, got.Type, got.Literal)
		}
	}
}

func TestLineAndColumn(t *testing.T) {
	input := "let a = 1\nlet b = 2\n"

	type pos struct {
		line, col int
		literal   string
	}
	want := []pos{
		{1, 1, "let"}, {1, 5, "a"}, {1, 7, "="}, {1, 9, "1"},
		{2, 1, "let"}, {2, 5, "b"}, {2, 7, "="}, {2, 9, "2"},
	}

	lex := NewLexer(input)
	for i, w := range want {
		got := lex.NextToken()
		if got.Line != w.line || got.Column != w.col {
			t.Fatalf("token %d (%s): want %d:%d, got %d:%d",
				i, got.Literal, w.line, w.col, got.Line, got.Column)
		}
	}
}

func TestStringEscapes(t *testing.T) {
	lex := NewLexer(`"line1\nline2\tend"`)
	got := lex.NextToken()
	if got.Type != STRING {
		t.Fatalf("want STRING, got %s", got.Type)
	}
	if got.Literal != "line1\nline2\tend" {
		t.Fatalf("escapes not resolved: %q", got.Literal)
	}
}

func TestUnterminatedStringIsIllegal(t *testing.T) {
	lex := NewLexer(`"oops`)
	got := lex.NextToken()
	if got.Type != ILLEGAL {
		t.Fatalf("want ILLEGAL for unterminated string, got %s", got.Type)
	}
}

func TestNumberSeparators(t *testing.T) {
	lex := NewLexer("1_000_000")
	got := lex.NextToken()
	if got.Type != INT || got.Literal != "1000000" {
		t.Fatalf("want INT 1000000, got %s %q", got.Type, got.Literal)
	}
}

func TestMultiLineString(t *testing.T) {
	lex := NewLexer(`"""hello
world"""`)
	got := lex.NextToken()
	if got.Type != STRING {
		t.Fatalf("want STRING, got %s", got.Type)
	}
	if got.Literal != "hello\nworld" {
		t.Fatalf("unexpected content: %q", got.Literal)
	}

	// Leading newline stripped, CRLF normalized to LF
	crlfLex := NewLexer("\"\"\"\r\nfirst\r\nsecond\r\n\"\"\"")
	got = crlfLex.NextToken()
	if got.Type != STRING {
		t.Fatalf("want STRING, got %s", got.Type)
	}
	if got.Literal != "first\nsecond\n" {
		t.Fatalf("unexpected CRLF content: %q", got.Literal)
	}

	unterminated := NewLexer(`"""no closing quotes`)
	got = unterminated.NextToken()
	if got.Type != ILLEGAL {
		t.Fatalf("want ILLEGAL for unterminated multi-line string, got %s", got.Type)
	}
	if got.Line != 1 || got.Column != 1 {
		t.Fatalf("unterminated string position: want 1:1, got %d:%d", got.Line, got.Column)
	}
}

// A byte order mark is not part of the program, and a file that starts with
// one should still run.
func TestByteOrderMarkIsSkipped(t *testing.T) {
	lex := NewLexer("\ufefflet x = 1")
	got := lex.NextToken()
	if got.Type != LET {
		t.Fatalf("want LET after a byte order mark, got %s", got.Type)
	}
}

// A string inside an {interpolation} must not end the string it sits in.
func TestInterpolationCanHoldAString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`"fib: {join(fibs, ", ")}"`, `fib: {join(fibs, ", ")}`},
		{`"fib: {join(fibs, \", \")}"`, `fib: {join(fibs, ", ")}`},
		{`"x: {upper("abc")}"`, `x: {upper("abc")}`},
	}

	for _, c := range cases {
		lex := NewLexer(c.input)
		got := lex.NextToken()
		if got.Type != STRING {
			t.Fatalf("%s: want STRING, got %s", c.input, got.Type)
		}
		if got.Literal != c.want {
			t.Fatalf("%s:\n want %q\n got  %q", c.input, c.want, got.Literal)
		}
	}
}

// "{{" is a literal brace, and both braces survive the lexer so that the
// split into text and expressions can turn them into one.
func TestDoubleBracesSurviveTheLexer(t *testing.T) {
	lex := NewLexer(`"{{literal}}"`)
	got := lex.NextToken()
	if got.Type != STRING {
		t.Fatalf("want STRING, got %s", got.Type)
	}
	if got.Literal != "{{literal}}" {
		t.Fatalf("want the braces kept, got %q", got.Literal)
	}
}
