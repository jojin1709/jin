package main

import "strings"

// Node is anything in the syntax tree.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement is a node that performs an action.
type Statement interface {
	Node
	statementNode()
}

// Expression is a node that produces a value.
type Expression interface {
	Node
	expressionNode()
}

// ---------------------------------------------------------------- program

// Program is the root node: a list of top-level statements.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var sb strings.Builder
	for _, s := range p.Statements {
		sb.WriteString(s.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------- types

// TypeAnnotation is a named type, e.g. int, str, bool.
type TypeAnnotation struct {
	Token Token
	Name  string
}

func (t *TypeAnnotation) String() string { return t.Name }

// Parameter is one entry in a function's parameter list.
type Parameter struct {
	Token Token
	Name  *Identifier
	Type  *TypeAnnotation
}

func (p *Parameter) String() string {
	if p.Type != nil {
		return p.Name.String() + ": " + p.Type.String()
	}
	return p.Name.String()
}

// ---------------------------------------------------------------- statements

// LetStatement declares a variable: let name: type = value
type LetStatement struct {
	Token Token
	Name  *Identifier
	Type  *TypeAnnotation
	Value Expression
}

func (s *LetStatement) statementNode()       {}
func (s *LetStatement) TokenLiteral() string { return s.Token.Literal }
func (s *LetStatement) String() string {
	var sb strings.Builder
	sb.WriteString("let ")
	sb.WriteString(s.Name.String())
	if s.Type != nil {
		sb.WriteString(": ")
		sb.WriteString(s.Type.String())
	}
	sb.WriteString(" = ")
	if s.Value != nil {
		sb.WriteString(s.Value.String())
	}
	return sb.String()
}

// FunctionStatement declares a named function.
type FunctionStatement struct {
	Token      Token
	Name       *Identifier
	Parameters []*Parameter
	ReturnType *TypeAnnotation
	Body       *BlockStatement
}

func (s *FunctionStatement) statementNode()       {}
func (s *FunctionStatement) TokenLiteral() string { return s.Token.Literal }
func (s *FunctionStatement) String() string {
	var sb strings.Builder
	sb.WriteString("fn ")
	sb.WriteString(s.Name.String())
	sb.WriteString("(")
	sb.WriteString(joinParams(s.Parameters))
	sb.WriteString(")")
	if s.ReturnType != nil {
		sb.WriteString(" -> ")
		sb.WriteString(s.ReturnType.String())
	}
	sb.WriteString(" ")
	if s.Body != nil {
		sb.WriteString(s.Body.String())
	}
	return sb.String()
}

// ImportStatement brings another file's names in behind an alias:
// import "utils" as u
type ImportStatement struct {
	Token Token
	Path  string
	Alias *Identifier
}

func (s *ImportStatement) statementNode()       {}
func (s *ImportStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ImportStatement) String() string {
	return "import \"" + s.Path + "\" as " + s.Alias.String()
}

// ReturnStatement returns a value from a function.
type ReturnStatement struct {
	Token Token
	Value Expression
}

func (s *ReturnStatement) statementNode()       {}
func (s *ReturnStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ReturnStatement) String() string {
	if s.Value == nil {
		return "return"
	}
	return "return " + s.Value.String()
}

// BlockStatement is a braced group of statements.
type BlockStatement struct {
	Token      Token
	Statements []Statement
}

func (s *BlockStatement) statementNode()       {}
func (s *BlockStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BlockStatement) String() string {
	parts := make([]string, 0, len(s.Statements))
	for _, st := range s.Statements {
		parts = append(parts, st.String())
	}
	return "{ " + strings.Join(parts, "; ") + " }"
}

// ExpressionStatement is an expression used as a statement, e.g. a call.
type ExpressionStatement struct {
	Token      Token
	Expression Expression
}

func (s *ExpressionStatement) statementNode()       {}
func (s *ExpressionStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ExpressionStatement) String() string {
	if s.Expression == nil {
		return ""
	}
	return s.Expression.String()
}

// IfStatement is if / else if / else.
type IfStatement struct {
	Token       Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative Statement // *BlockStatement or *IfStatement
}

func (s *IfStatement) statementNode()       {}
func (s *IfStatement) TokenLiteral() string { return s.Token.Literal }
func (s *IfStatement) String() string {
	var sb strings.Builder
	sb.WriteString("if ")
	if s.Condition != nil {
		sb.WriteString(s.Condition.String())
	}
	sb.WriteString(" ")
	if s.Consequence != nil {
		sb.WriteString(s.Consequence.String())
	}
	if s.Alternative != nil {
		sb.WriteString(" else ")
		sb.WriteString(s.Alternative.String())
	}
	return sb.String()
}

// ForStatement iterates: for x in iterable { }
type ForStatement struct {
	Token    Token
	VarName  *Identifier
	Iterable Expression
	Body     *BlockStatement
}

func (s *ForStatement) statementNode()       {}
func (s *ForStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ForStatement) String() string {
	var sb strings.Builder
	sb.WriteString("for ")
	sb.WriteString(s.VarName.String())
	sb.WriteString(" in ")
	if s.Iterable != nil {
		sb.WriteString(s.Iterable.String())
	}
	sb.WriteString(" ")
	if s.Body != nil {
		sb.WriteString(s.Body.String())
	}
	return sb.String()
}

// WhileStatement loops while a condition holds.
type WhileStatement struct {
	Token     Token
	Condition Expression
	Body      *BlockStatement
}

func (s *WhileStatement) statementNode()       {}
func (s *WhileStatement) TokenLiteral() string { return s.Token.Literal }
func (s *WhileStatement) String() string {
	var sb strings.Builder
	sb.WriteString("while ")
	if s.Condition != nil {
		sb.WriteString(s.Condition.String())
	}
	sb.WriteString(" ")
	if s.Body != nil {
		sb.WriteString(s.Body.String())
	}
	return sb.String()
}

// BreakStatement exits the nearest loop.
type BreakStatement struct{ Token Token }

func (s *BreakStatement) statementNode()       {}
func (s *BreakStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BreakStatement) String() string       { return "break" }

// ContinueStatement skips to the next loop iteration.
type ContinueStatement struct{ Token Token }

func (s *ContinueStatement) statementNode()       {}
func (s *ContinueStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ContinueStatement) String() string       { return "continue" }

// ---------------------------------------------------------------- expressions

// Identifier is a variable or function name.
type Identifier struct {
	Token Token
	Value string
}

func (e *Identifier) expressionNode()      {}
func (e *Identifier) TokenLiteral() string { return e.Token.Literal }
func (e *Identifier) String() string       { return e.Value }

// IntegerLiteral is a whole number.
type IntegerLiteral struct {
	Token Token
	Value int64
}

func (e *IntegerLiteral) expressionNode()      {}
func (e *IntegerLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *IntegerLiteral) String() string       { return e.Token.Literal }

// FloatLiteral is a decimal number.
type FloatLiteral struct {
	Token Token
	Value float64
}

func (e *FloatLiteral) expressionNode()      {}
func (e *FloatLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *FloatLiteral) String() string       { return e.Token.Literal }

// StringLiteral is text. Interpolation is resolved at runtime.
type StringLiteral struct {
	Token Token
	Value string
}

func (e *StringLiteral) expressionNode()      {}
func (e *StringLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *StringLiteral) String() string       { return `"` + e.Value + `"` }

// BooleanLiteral is true or false.
type BooleanLiteral struct {
	Token Token
	Value bool
}

func (e *BooleanLiteral) expressionNode()      {}
func (e *BooleanLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *BooleanLiteral) String() string       { return e.Token.Literal }

// NilLiteral is the absence of a value.
type NilLiteral struct{ Token Token }

func (e *NilLiteral) expressionNode()      {}
func (e *NilLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *NilLiteral) String() string       { return "nil" }

// PrefixExpression is -x or !x.
type PrefixExpression struct {
	Token    Token
	Operator string
	Right    Expression
}

func (e *PrefixExpression) expressionNode()      {}
func (e *PrefixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *PrefixExpression) String() string {
	return "(" + e.Operator + e.Right.String() + ")"
}

// InfixExpression is a binary operation like a + b.
type InfixExpression struct {
	Token    Token
	Left     Expression
	Operator string
	Right    Expression
}

func (e *InfixExpression) expressionNode()      {}
func (e *InfixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *InfixExpression) String() string {
	return "(" + e.Left.String() + " " + e.Operator + " " + e.Right.String() + ")"
}

// AssignExpression assigns a value: target = value
type AssignExpression struct {
	Token  Token
	Target Expression
	Value  Expression
}

func (e *AssignExpression) expressionNode()      {}
func (e *AssignExpression) TokenLiteral() string { return e.Token.Literal }
func (e *AssignExpression) String() string {
	return "(" + e.Target.String() + " = " + e.Value.String() + ")"
}

// CallExpression calls a function: f(a, b)
type CallExpression struct {
	Token     Token
	Function  Expression
	Arguments []Expression
}

func (e *CallExpression) expressionNode()      {}
func (e *CallExpression) TokenLiteral() string { return e.Token.Literal }
func (e *CallExpression) String() string {
	args := make([]string, 0, len(e.Arguments))
	for _, a := range e.Arguments {
		args = append(args, a.String())
	}
	return e.Function.String() + "(" + strings.Join(args, ", ") + ")"
}

// IndexExpression reads an element: list[0]
type IndexExpression struct {
	Token Token
	Left  Expression
	Index Expression
}

func (e *IndexExpression) expressionNode()      {}
func (e *IndexExpression) TokenLiteral() string { return e.Token.Literal }
func (e *IndexExpression) String() string {
	return "(" + e.Left.String() + "[" + e.Index.String() + "])"
}

// MemberExpression reads a property: obj.name
type MemberExpression struct {
	Token    Token
	Object   Expression
	Property *Identifier
}

func (e *MemberExpression) expressionNode()      {}
func (e *MemberExpression) TokenLiteral() string { return e.Token.Literal }
func (e *MemberExpression) String() string {
	return e.Object.String() + "." + e.Property.String()
}

// ArrayLiteral builds a list: [1, 2, 3]
type ArrayLiteral struct {
	Token    Token
	Elements []Expression
}

func (e *ArrayLiteral) expressionNode()      {}
func (e *ArrayLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *ArrayLiteral) String() string {
	parts := make([]string, 0, len(e.Elements))
	for _, el := range e.Elements {
		parts = append(parts, el.String())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// FunctionLiteral is an anonymous function: fn(a) { }
type FunctionLiteral struct {
	Token      Token
	Parameters []*Parameter
	ReturnType *TypeAnnotation
	Body       *BlockStatement
}

func (e *FunctionLiteral) expressionNode()      {}
func (e *FunctionLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *FunctionLiteral) String() string {
	var sb strings.Builder
	sb.WriteString("fn(")
	sb.WriteString(joinParams(e.Parameters))
	sb.WriteString(")")
	if e.ReturnType != nil {
		sb.WriteString(" -> ")
		sb.WriteString(e.ReturnType.String())
	}
	sb.WriteString(" ")
	if e.Body != nil {
		sb.WriteString(e.Body.String())
	}
	return sb.String()
}

func joinParams(params []*Parameter) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ", ")
}

// MapPair is one key and value written inside a map literal.
type MapPair struct {
	Key   Expression
	Value Expression
}

// MapLiteral is a map written out in full, such as {"a": 1, "b": 2}.
type MapLiteral struct {
	Token Token
	Pairs []MapPair
}

func (e *MapLiteral) expressionNode()      {}
func (e *MapLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *MapLiteral) String() string {
	parts := make([]string, 0, len(e.Pairs))
	for _, pair := range e.Pairs {
		parts = append(parts, pair.Key.String()+": "+pair.Value.String())
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
