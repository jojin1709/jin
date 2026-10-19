package main

import (
	"fmt"
	"strconv"
)

// Operator precedence. Higher binds tighter.
const (
	_ int = iota
	PREC_LOWEST
	PREC_ASSIGN // =
	PREC_OR     // ||
	PREC_AND    // &&
	PREC_EQUALS // == !=
	PREC_LESSGREATER
	PREC_SUM     // + -
	PREC_PRODUCT // * / %
	PREC_PREFIX  // -x !x
	PREC_CALL    // f() a[i] a.b
)

var precedences = map[TokenType]int{
	ASSIGN:   PREC_ASSIGN,
	OR:       PREC_OR,
	AND:      PREC_AND,
	EQ:       PREC_EQUALS,
	NOT_EQ:   PREC_EQUALS,
	LT:       PREC_LESSGREATER,
	GT:       PREC_LESSGREATER,
	LE:       PREC_LESSGREATER,
	GE:       PREC_LESSGREATER,
	PLUS:     PREC_SUM,
	MINUS:    PREC_SUM,
	SLASH:    PREC_PRODUCT,
	ASTERISK: PREC_PRODUCT,
	PERCENT:  PREC_PRODUCT,
	LPAREN:   PREC_CALL,
	LBRACKET: PREC_CALL,
	DOT:      PREC_CALL,
	LBRACE:   PREC_CALL,
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

// Parser turns a token stream into a syntax tree.
type Parser struct {
	l      *Lexer
	errors []string

	curToken  Token
	peekToken Token

	noStructLiteral bool

	prefixParseFns map[TokenType]prefixParseFn
	infixParseFns  map[TokenType]infixParseFn
}

// NewParser builds a parser and primes its token lookahead.
func NewParser(l *Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	p.prefixParseFns = map[TokenType]prefixParseFn{
		IDENT:    p.parseIdentifier,
		INT:      p.parseIntegerLiteral,
		FLOAT:    p.parseFloatLiteral,
		STRING:   p.parseStringLiteral,
		TRUE:     p.parseBoolean,
		FALSE:    p.parseBoolean,
		NIL:      p.parseNil,
		BANG:     p.parsePrefixExpression,
		MINUS:    p.parsePrefixExpression,
		LPAREN:   p.parseGroupedExpression,
		LBRACKET: p.parseArrayLiteral,
		LBRACE:   p.parseMapLiteral,
		FUNCTION: p.parseFunctionLiteral,
	}

	p.infixParseFns = map[TokenType]infixParseFn{
		PLUS:     p.parseInfixExpression,
		MINUS:    p.parseInfixExpression,
		ASTERISK: p.parseInfixExpression,
		SLASH:    p.parseInfixExpression,
		PERCENT:  p.parseInfixExpression,
		EQ:       p.parseInfixExpression,
		NOT_EQ:   p.parseInfixExpression,
		LT:       p.parseInfixExpression,
		GT:       p.parseInfixExpression,
		LE:       p.parseInfixExpression,
		GE:       p.parseInfixExpression,
		AND:      p.parseInfixExpression,
		OR:       p.parseInfixExpression,
		ASSIGN:   p.parseAssignExpression,
		LPAREN:   p.parseCallExpression,
		LBRACKET: p.parseIndexExpression,
		DOT:      p.parseMemberExpression,
		LBRACE:   p.parseStructLiteral,
	}

	// Prime curToken and peekToken.
	p.nextToken()
	p.nextToken()
	return p
}

// Errors returns the parse errors collected so far.
func (p *Parser) Errors() []string { return p.errors }

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t TokenType) bool  { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t TokenType) bool { return p.peekToken.Type == t }

func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.errorf("expected %s, got %s", t, p.peekToken.Type)
	return false
}

func (p *Parser) peekPrecedence() int {
	if p.peekToken.Type == LBRACE && p.noStructLiteral {
		return PREC_LOWEST
	}
	if pr, ok := precedences[p.peekToken.Type]; ok {
		return pr
	}
	return PREC_LOWEST
}

func (p *Parser) curPrecedence() int {
	if pr, ok := precedences[p.curToken.Type]; ok {
		return pr
	}
	return PREC_LOWEST
}

func (p *Parser) errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.errors = append(p.errors, fmt.Sprintf("%s: %s", p.curToken.Position(), msg))
}

// ---------------------------------------------------------------- program

// ParseProgram parses the whole input into a Program.
func (p *Parser) ParseProgram() *Program {
	program := &Program{Statements: []Statement{}}

	for !p.curTokenIs(EOF) {
		// A ';' separating statements is not a statement of its own. It needs
		// skipping here as well as in a block, because a statement ending in a
		// brace does not eat its own semicolon.
		if p.curTokenIs(SEMICOLON) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() Statement {
	switch p.curToken.Type {
	case IMPORT:
		return p.parseImportStatement()
	case STRUCT:
		return p.parseStructStatement()
	case LET:
		return p.parseLetStatement()
	case FUNCTION:
		return p.parseFunctionStatement()
	case RETURN:
		return p.parseReturnStatement()
	case IF:
		return p.parseIfStatement()
	case FOR:
		return p.parseForStatement()
	case WHILE:
		return p.parseWhileStatement()
	case BREAK:
		return p.parseBreakStatement()
	case CONTINUE:
		return p.parseContinueStatement()
	case TRY:
		return p.parseTryStatement()
	default:
		return p.parseExpressionStatement()
	}
}

// parseImportStatement reads `import "path" as alias`. The path is a file's
// name without the .jin ending, and the alias is required: a file's names are
// always reached through it, so a name from another file is never a surprise.
func (p *Parser) parseImportStatement() *ImportStatement {
	stmt := &ImportStatement{Token: p.curToken}

	if !p.expectPeek(STRING) {
		return nil
	}
	stmt.Path = p.curToken.Literal

	if !p.expectPeek(AS) {
		return nil
	}
	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Alias = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	return stmt
}

// parseStructStatement reads a record type:
//
//	struct Point { x: int, y: int }
//
// Fields may be separated by a comma, or by nothing at all — a newline is a
// separator in Jin, as it is anywhere else. The closing brace is left in
// curToken, the way every other braced statement leaves it.
func (p *Parser) parseStructStatement() *StructStatement {
	stmt := &StructStatement{Token: p.curToken, Fields: []*Field{}}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(LBRACE) {
		return nil
	}

	p.nextToken() // move onto the first field, or the closing brace
	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
		if p.curTokenIs(COMMA) || p.curTokenIs(SEMICOLON) {
			p.nextToken()
			continue
		}
		if !p.curTokenIs(IDENT) {
			p.errorf("expected a field name, got %s", p.curToken.Type)
			return nil
		}

		field := &Field{
			Token: p.curToken,
			Name:  &Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}
		if !p.expectPeek(COLON) {
			return nil
		}
		if !p.expectPeek(IDENT) {
			return nil
		}
		field.Type = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
		stmt.Fields = append(stmt.Fields, field)

		p.nextToken() // move onto the separator, the next field, or '}'
	}

	if !p.curTokenIs(RBRACE) {
		p.errorf("expected }, got %s", p.curToken.Type)
	}
	return stmt
}

func (p *Parser) parseLetStatement() *LetStatement {
	stmt := &LetStatement{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(COLON) {
		p.nextToken() // move to ':'
		if !p.expectPeek(IDENT) {
			return nil
		}
		stmt.Type = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
	}

	if !p.expectPeek(ASSIGN) {
		return nil
	}

	p.nextToken() // move onto the first token of the value
	stmt.Value = p.parseExpression(PREC_LOWEST)

	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseFunctionStatement() *FunctionStatement {
	stmt := &FunctionStatement{Token: p.curToken}

	if p.peekTokenIs(LPAREN) {
		p.nextToken() // move onto '('
		p.nextToken() // move onto the receiver parameter name
		if !p.curTokenIs(IDENT) {
			p.errorf("expected a receiver name, got %s", p.curToken.Type)
			return nil
		}
		receiver := &Parameter{
			Token: p.curToken,
			Name:  &Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}
		if !p.expectPeek(COLON) {
			return nil
		}
		if !p.expectPeek(IDENT) {
			return nil
		}
		receiver.Type = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
		if !p.expectPeek(RPAREN) {
			return nil
		}
		stmt.Receiver = receiver
	}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(LPAREN) {
		return nil
	}
	stmt.Parameters = p.parseFunctionParameters()

	if p.peekTokenIs(ARROW) {
		p.nextToken() // move to '->'
		if !p.expectPeek(IDENT) {
			return nil
		}
		stmt.ReturnType = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

// parseFunctionParameters assumes curToken is '(' and leaves it on ')'.
func (p *Parser) parseFunctionParameters() []*Parameter {
	params := []*Parameter{}

	if p.peekTokenIs(RPAREN) {
		p.nextToken()
		return params
	}

	for {
		p.nextToken() // move onto the parameter name
		if !p.curTokenIs(IDENT) {
			p.errorf("expected parameter name, got %s", p.curToken.Type)
			return nil
		}

		param := &Parameter{
			Token: p.curToken,
			Name:  &Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}

		if p.peekTokenIs(COLON) {
			p.nextToken() // move to ':'
			if !p.expectPeek(IDENT) {
				return nil
			}
			param.Type = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
		}

		params = append(params, param)

		if p.peekTokenIs(COMMA) {
			p.nextToken() // consume ','
			continue
		}
		break
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}
	return params
}

func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.curToken}

	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
		return stmt
	}
	if p.peekTokenIs(RBRACE) || p.peekTokenIs(EOF) {
		return stmt // bare return
	}

	p.nextToken()
	stmt.Value = p.parseExpression(PREC_LOWEST)

	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseIfStatement() Statement {
	stmt := &IfStatement{Token: p.curToken}

	p.nextToken()
	p.noStructLiteral = true
	stmt.Condition = p.parseExpression(PREC_LOWEST)
	p.noStructLiteral = false

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(ELSE) {
		p.nextToken() // move to 'else'

		if p.peekTokenIs(IF) {
			p.nextToken() // move to 'if'
			stmt.Alternative = p.parseIfStatement()
			return stmt
		}

		if !p.expectPeek(LBRACE) {
			return nil
		}
		stmt.Alternative = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseForStatement() *ForStatement {
	stmt := &ForStatement{Token: p.curToken}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.VarName = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(IN) {
		return nil
	}

	p.nextToken()
	p.noStructLiteral = true
	stmt.Iterable = p.parseExpression(PREC_LOWEST)
	p.noStructLiteral = false

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.curToken}

	p.nextToken()
	p.noStructLiteral = true
	stmt.Condition = p.parseExpression(PREC_LOWEST)
	p.noStructLiteral = false

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseBreakStatement() *BreakStatement {
	stmt := &BreakStatement{Token: p.curToken}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseContinueStatement() *ContinueStatement {
	stmt := &ContinueStatement{Token: p.curToken}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseTryStatement() *TryStatement {
	stmt := &TryStatement{Token: p.curToken}

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.TryBody = p.parseBlockStatement()

	if !p.expectPeek(CATCH) {
		return nil
	}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.CatchIdent = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.CatchBody = p.parseBlockStatement()

	return stmt
}

// parseBlockStatement assumes curToken is '{' and leaves it on '}'.
func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}

	p.nextToken()
	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
		// Statements that end in a brace do not eat their own ';', so one
		// left over from them is a separator and gets skipped here.
		if p.curTokenIs(SEMICOLON) {
			p.nextToken()
			continue
		}
		// An import belongs to a file, not to a block inside one.
		if p.curTokenIs(IMPORT) {
			p.errorf("'import' is only allowed at the top level of a file")
			p.parseImportStatement()
			p.nextToken()
			continue
		}
		// A struct is a named type, and a type belongs to the file it is
		// written in, so it cannot be declared inside a block.
		if p.curTokenIs(STRUCT) {
			p.errorf("'struct' is only allowed at the top level of a file")
			p.parseStructStatement()
			p.nextToken()
			continue
		}
		// Methods are attached to a type, so they belong at the top level.
		if p.curTokenIs(FUNCTION) && p.peekTokenIs(LPAREN) {
			p.errorf("methods are only allowed at the top level of a file")
			p.parseFunctionStatement()
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(PREC_LOWEST)
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

// ---------------------------------------------------------------- expressions

func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.errorf("unexpected token %q", p.curToken.Literal)
		return nil
	}
	left := prefix()

	for !p.peekTokenIs(SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return left
		}
		p.nextToken()
		left = infix(left)
	}

	return left
}

func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() Expression {
	lit := &IntegerLiteral{Token: p.curToken}
	value, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.errorf("could not parse %q as an integer", p.curToken.Literal)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() Expression {
	lit := &FloatLiteral{Token: p.curToken}
	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.errorf("could not parse %q as a float", p.curToken.Literal)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() Expression {
	return &StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolean() Expression {
	return &BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(TRUE)}
}

func (p *Parser) parseNil() Expression {
	return &NilLiteral{Token: p.curToken}
}

func (p *Parser) parsePrefixExpression() Expression {
	exp := &PrefixExpression{Token: p.curToken, Operator: p.curToken.Literal}
	p.nextToken()
	exp.Right = p.parseExpression(PREC_PREFIX)
	return exp
}

func (p *Parser) parseInfixExpression(left Expression) Expression {
	exp := &InfixExpression{
		Token:    p.curToken,
		Left:     left,
		Operator: p.curToken.Literal,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	exp.Right = p.parseExpression(precedence)
	return exp
}

func (p *Parser) parseAssignExpression(left Expression) Expression {
	exp := &AssignExpression{Token: p.curToken, Target: left}
	p.nextToken()
	// Right-associative: a = b = c parses as a = (b = c)
	exp.Value = p.parseExpression(PREC_ASSIGN - 1)
	return exp
}

func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()
	saved := p.noStructLiteral
	p.noStructLiteral = false
	exp := p.parseExpression(PREC_LOWEST)
	p.noStructLiteral = saved
	if !p.expectPeek(RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseCallExpression(fn Expression) Expression {
	exp := &CallExpression{Token: p.curToken, Function: fn}
	exp.Arguments = p.parseExpressionList(RPAREN)
	return exp
}

func (p *Parser) parseIndexExpression(left Expression) Expression {
	exp := &IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(PREC_LOWEST)
	if !p.expectPeek(RBRACKET) {
		return nil
	}
	return exp
}

func (p *Parser) parseMemberExpression(left Expression) Expression {
	dot := p.curToken
	if !p.expectPeek(IDENT) {
		return nil
	}
	return &MemberExpression{
		Token:    dot,
		Object:   left,
		Property: &Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}
}

func (p *Parser) parseArrayLiteral() Expression {
	arr := &ArrayLiteral{Token: p.curToken}
	arr.Elements = p.parseExpressionList(RBRACKET)
	return arr
}

// parseMapLiteral reads {"key": value, ...}. A bare `{` only reaches here in
// expression position; where a statement can start, `{` still opens a block.
func (p *Parser) parseMapLiteral() Expression {
	lit := &MapLiteral{Token: p.curToken, Pairs: []MapPair{}}

	if p.peekTokenIs(RBRACE) {
		p.nextToken() // move onto '}'
		return lit
	}

	p.nextToken() // move onto the first key
	for {
		key := p.parseExpression(PREC_LOWEST)
		if key == nil {
			return nil
		}

		if !p.expectPeek(COLON) {
			return nil
		}
		p.nextToken() // move onto the value

		value := p.parseExpression(PREC_LOWEST)
		if value == nil {
			return nil
		}
		lit.Pairs = append(lit.Pairs, MapPair{Key: key, Value: value})

		if p.peekTokenIs(RBRACE) {
			p.nextToken()
			return lit
		}
		if !p.expectPeek(COMMA) {
			return nil
		}
		p.nextToken() // move onto the next key
	}
}

func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()

	if p.peekTokenIs(ARROW) {
		p.nextToken()
		if !p.expectPeek(IDENT) {
			return nil
		}
		lit.ReturnType = &TypeAnnotation{Token: p.curToken, Name: p.curToken.Literal}
	}

	if !p.expectPeek(LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()
	return lit
}

// parseExpressionList assumes curToken is '(' or '[' and leaves it on the
// matching closer.
func (p *Parser) parseExpressionList(end TokenType) []Expression {
	list := []Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(PREC_LOWEST))

	for p.peekTokenIs(COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(PREC_LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}
	return list
}

func (p *Parser) parseStructLiteral(rec Expression) Expression {
	lit := &StructLiteral{
		Token:  p.curToken,
		Struct: rec,
		Fields: []*StructFieldInit{},
	}

	if p.peekTokenIs(RBRACE) {
		p.nextToken()
		return lit
	}

	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
		if !p.expectPeek(IDENT) {
			return nil
		}
		name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}

		if !p.expectPeek(COLON) {
			return nil
		}

		p.nextToken()
		val := p.parseExpression(PREC_LOWEST)
		if val == nil {
			return nil
		}

		lit.Fields = append(lit.Fields, &StructFieldInit{
			Name:  name,
			Value: val,
		})

		if p.peekTokenIs(COMMA) {
			p.nextToken()
			if p.peekTokenIs(RBRACE) {
				p.nextToken()
				break
			}
			continue
		}

		if p.peekTokenIs(RBRACE) {
			p.nextToken()
			break
		}
		p.errorf("expected ',' or '}', got %s", p.peekToken.Type)
		return nil
	}

	return lit
}
