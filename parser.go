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
	p.errors = append(p.errors, fmt.Sprintf("%d:%d: %s", p.curToken.Line, p.curToken.Column, msg))
}

// ---------------------------------------------------------------- program

// ParseProgram parses the whole input into a Program.
func (p *Parser) ParseProgram() *Program {
	program := &Program{Statements: []Statement{}}

	for !p.curTokenIs(EOF) {
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
	default:
		return p.parseExpressionStatement()
	}
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
	stmt.Condition = p.parseExpression(PREC_LOWEST)

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
	stmt.Iterable = p.parseExpression(PREC_LOWEST)

	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.curToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(PREC_LOWEST)

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

// parseBlockStatement assumes curToken is '{' and leaves it on '}'.
func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}

	p.nextToken()
	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
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
	exp := p.parseExpression(PREC_LOWEST)
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
