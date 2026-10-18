package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- types

// TypeKind is the shape of a type.
type TypeKind int

const (
	TypeAny TypeKind = iota
	TypeInt
	TypeFloat
	TypeStr
	TypeBool
	TypeNil
	TypeArray
	TypeFn
)

// Type describes the type of a value.
type Type struct {
	Kind     TypeKind
	Elem     *Type   // arrays
	Params   []*Type // functions
	Ret      *Type   // functions
	Variadic bool
}

var (
	anyT   = &Type{Kind: TypeAny}
	intT   = &Type{Kind: TypeInt}
	floatT = &Type{Kind: TypeFloat}
	strT   = &Type{Kind: TypeStr}
	boolT  = &Type{Kind: TypeBool}
	nilT   = &Type{Kind: TypeNil}
)

func arrayOf(elem *Type) *Type { return &Type{Kind: TypeArray, Elem: elem} }

// fnOf builds a function type. A nil return type is displayed as any because
// an unannotated function is not return-checked.
func fnOf(params []*Type, ret *Type) *Type {
	if ret == nil {
		ret = anyT
	}
	return &Type{Kind: TypeFn, Params: params, Ret: ret}
}

func (t *Type) String() string {
	switch t.Kind {
	case TypeAny:
		return "any"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeStr:
		return "str"
	case TypeBool:
		return "bool"
	case TypeNil:
		return "nil"
	case TypeArray:
		if t.Elem == nil || t.Elem.Kind == TypeAny {
			return "array"
		}
		return "array<" + t.Elem.String() + ">"
	case TypeFn:
		parts := make([]string, 0, len(t.Params))
		for _, p := range t.Params {
			parts = append(parts, p.String())
		}
		if t.Variadic && len(parts) > 0 {
			parts[len(parts)-1] += "..."
		}
		return "fn(" + strings.Join(parts, ", ") + ") -> " + t.Ret.String()
	}
	return "?"
}

// assignable reports whether a value of type value fits a slot of type target.
// any is compatible with everything, and nil is currently allowed anywhere
// because Jin has no optional types yet.
func assignable(target, value *Type) bool {
	if target == nil || value == nil {
		return true
	}
	if target.Kind == TypeAny || value.Kind == TypeAny {
		return true
	}
	if value.Kind == TypeNil {
		return true
	}
	if target.Kind != value.Kind {
		return false
	}
	switch target.Kind {
	case TypeArray:
		return assignable(target.Elem, value.Elem)
	case TypeFn:
		if len(target.Params) != len(value.Params) {
			return false
		}
		for i := range target.Params {
			if !assignable(target.Params[i], value.Params[i]) {
				return false
			}
		}
		return assignable(target.Ret, value.Ret)
	}
	return true
}

// unify returns the most specific type that fits both values.
func unify(a, b *Type) *Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Kind == TypeAny || b.Kind == TypeAny {
		return anyT
	}
	if a.Kind != b.Kind {
		return anyT
	}
	if a.Kind == TypeArray {
		return arrayOf(unify(a.Elem, b.Elem))
	}
	return a
}

func isBoolish(t *Type) bool { return t.Kind == TypeBool || t.Kind == TypeAny }

func isNumericKind(k TypeKind) bool { return k == TypeInt || k == TypeFloat }

// ---------------------------------------------------------------- checker

// checkError is one problem found by the checker, with its position.
type checkError struct {
	Line int
	Col  int
	Msg  string
}

// Checker walks a syntax tree and reports type errors before the program runs.
type Checker struct {
	errors   []checkError
	scopes   []map[string]*Type
	builtins map[string]*builtinSig

	fnRet     *Type // nil when the function has no declared return type
	loopDepth int
	fnDepth   int
}

// builtinSig describes a built-in function for the checker.
type builtinSig struct {
	Check func(c *Checker, args []*Type, tok Token) *Type
}

// NewChecker returns a checker with an empty top-level scope.
func NewChecker() *Checker {
	return &Checker{
		errors:   []checkError{},
		scopes:   []map[string]*Type{{}},
		builtins: newBuiltinSigs(),
	}
}

// Check type-checks a whole program and returns every error it found, sorted
// by position so a file's problems read top to bottom.
func Check(program *Program) []string {
	return NewChecker().CheckProgram(program)
}

// CheckProgram type-checks a program against this checker's scopes and returns
// the errors. Calling it again keeps the declarations from earlier calls, which
// is what the REPL relies on to remember types between lines.
func (c *Checker) CheckProgram(program *Program) []string {
	c.errors = c.errors[:0]
	c.fnRet = nil
	c.loopDepth = 0
	c.fnDepth = 0

	// Functions are registered before their bodies are checked so that
	// functions can call each other in any order.
	c.predeclare(program)

	// Top-level function bodies are checked after the rest of the file, so a
	// helper defined at the top can use a variable declared further down.
	// Top-level code itself still has to declare a name before using it.
	var deferred []*FunctionStatement
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*FunctionStatement); ok {
			deferred = append(deferred, fn)
			continue
		}
		c.checkStatement(stmt)
	}
	for _, fn := range deferred {
		c.checkFunctionBody(fn.Name.Value, fn.Parameters, fn.ReturnType, fn.Body, fn.Token)
	}

	sort.SliceStable(c.errors, func(i, j int) bool {
		if c.errors[i].Line != c.errors[j].Line {
			return c.errors[i].Line < c.errors[j].Line
		}
		return c.errors[i].Col < c.errors[j].Col
	})

	out := make([]string, 0, len(c.errors))
	for _, e := range c.errors {
		out = append(out, fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg))
	}
	return out
}

// GlobalScope returns a copy of the top-level type scope, so a caller can undo
// the declarations made by a failed check.
func (c *Checker) GlobalScope() map[string]*Type {
	out := make(map[string]*Type, len(c.scopes[0]))
	for name, t := range c.scopes[0] {
		out[name] = t
	}
	return out
}

// SetGlobalScope replaces the top-level type scope.
func (c *Checker) SetGlobalScope(scope map[string]*Type) {
	c.scopes[0] = scope
}

func (c *Checker) errorf(tok Token, format string, args ...any) {
	c.errors = append(c.errors, checkError{
		Line: tok.Line,
		Col:  tok.Column,
		Msg:  fmt.Sprintf(format, args...),
	})
}

func (c *Checker) pushScope() { c.scopes = append(c.scopes, map[string]*Type{}) }
func (c *Checker) popScope()  { c.scopes = c.scopes[:len(c.scopes)-1] }

func (c *Checker) declare(name string, t *Type) {
	c.scopes[len(c.scopes)-1][name] = t
}

func (c *Checker) lookup(name string) (*Type, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if t, ok := c.scopes[i][name]; ok {
			return t, true
		}
	}
	return nil, false
}

// predeclare registers every top-level function so calls can go in any order.
func (c *Checker) predeclare(program *Program) {
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*FunctionStatement); ok {
			c.declare(fn.Name.Value, c.fnSignature(fn.Parameters, fn.ReturnType))
		}
	}
}

// annotationType turns a written type name into a Type.
func (c *Checker) annotationType(ann *TypeAnnotation) *Type {
	if ann == nil {
		return anyT
	}
	switch ann.Name {
	case "int":
		return intT
	case "float":
		return floatT
	case "str":
		return strT
	case "bool":
		return boolT
	case "nil":
		return nilT
	case "array":
		// Element types are inferred; a bare "array" means array<any>.
		return arrayOf(anyT)
	case "any":
		return anyT
	}
	c.errorf(ann.Token, "unknown type %q", ann.Name)
	return anyT
}

// optionalAnnotationType returns nil when no annotation was written, so the
// caller can tell "no annotation" apart from "annotated any".
func (c *Checker) optionalAnnotationType(ann *TypeAnnotation) *Type {
	if ann == nil {
		return nil
	}
	return c.annotationType(ann)
}

func (c *Checker) fnSignature(params []*Parameter, retAnn *TypeAnnotation) *Type {
	ps := make([]*Type, 0, len(params))
	for _, p := range params {
		ps = append(ps, c.annotationType(p.Type))
	}
	return fnOf(ps, c.optionalAnnotationType(retAnn))
}

// ---------------------------------------------------------------- statements

func (c *Checker) checkStatement(stmt Statement) *Type {
	switch node := stmt.(type) {

	case *ExpressionStatement:
		return c.checkExpression(node.Expression)

	case *LetStatement:
		declared := c.optionalAnnotationType(node.Type)
		valueType := c.checkExpression(node.Value)
		if declared != nil && !assignable(declared, valueType) {
			c.errorf(node.Token, "cannot assign %s to %s variable %q", valueType, declared, node.Name.Value)
		}
		if declared == nil {
			declared = valueType
		}
		c.declare(node.Name.Value, declared)
		return declared

	case *ReturnStatement:
		if c.fnDepth == 0 {
			c.errorf(node.Token, "'return' is only allowed inside a function")
			return nilT
		}
		if node.Value == nil {
			return nilT
		}
		valueType := c.checkExpression(node.Value)
		if c.fnRet != nil && !assignable(c.fnRet, valueType) {
			c.errorf(node.Token, "cannot return %s from a function declared to return %s", valueType, c.fnRet)
		}
		return valueType

	case *FunctionStatement:
		sig := c.fnSignature(node.Parameters, node.ReturnType)
		c.declare(node.Name.Value, sig)
		c.checkFunctionBody(node.Name.Value, node.Parameters, node.ReturnType, node.Body, node.Token)
		return sig

	case *IfStatement:
		condType := c.checkExpression(node.Condition)
		if !isBoolish(condType) {
			c.errorf(node.Token, "condition must be a bool, got %s", condType)
		}
		thenType := c.checkBlock(node.Consequence)
		elseType := nilT
		if node.Alternative != nil {
			if block, ok := node.Alternative.(*BlockStatement); ok {
				elseType = c.checkBlock(block)
			} else {
				elseType = c.checkStatement(node.Alternative)
			}
		}
		return unify(thenType, elseType)

	case *ForStatement:
		iterType := c.checkExpression(node.Iterable)
		elemType := anyT
		switch iterType.Kind {
		case TypeArray:
			if iterType.Elem != nil {
				elemType = iterType.Elem
			}
		case TypeStr:
			elemType = strT
		case TypeAny:
			elemType = anyT
		default:
			c.errorf(node.Token, "cannot loop over %s", iterType)
		}

		c.loopDepth++
		c.pushScope()
		c.declare(node.VarName.Value, elemType)
		c.checkBlockNoScope(node.Body)
		c.popScope()
		c.loopDepth--
		return nilT

	case *WhileStatement:
		condType := c.checkExpression(node.Condition)
		if !isBoolish(condType) {
			c.errorf(node.Token, "condition must be a bool, got %s", condType)
		}
		c.loopDepth++
		c.checkBlock(node.Body)
		c.loopDepth--
		return nilT

	case *BreakStatement:
		if c.loopDepth == 0 {
			c.errorf(node.Token, "'break' is only allowed inside a loop")
		}
		return nilT

	case *ContinueStatement:
		if c.loopDepth == 0 {
			c.errorf(node.Token, "'continue' is only allowed inside a loop")
		}
		return nilT

	case *BlockStatement:
		return c.checkBlock(node)
	}

	return anyT
}

// checkFunctionBody checks a function body with its parameters in scope.
func (c *Checker) checkFunctionBody(name string, params []*Parameter, retAnn *TypeAnnotation, body *BlockStatement, tok Token) *Type {
	sig := c.fnSignature(params, retAnn)

	savedRet, savedLoop, savedDepth := c.fnRet, c.loopDepth, c.fnDepth
	c.fnRet = c.optionalAnnotationType(retAnn)
	c.loopDepth = 0 // a nested function does not inherit the outer loop
	c.fnDepth++

	c.pushScope()
	for i, p := range params {
		c.declare(p.Name.Value, sig.Params[i])
	}
	bodyType := c.checkBlockNoScope(body)
	c.popScope()

	// Implicit return: with no explicit return anywhere, the last expression is
	// the result, so it has to match the declared return type.
	if c.fnRet != nil && !containsReturn(body) && !assignable(c.fnRet, bodyType) {
		c.errorf(tok, "%s returns %s on its last line but is declared to return %s",
			describeFn(name), bodyType, c.fnRet)
	}

	c.fnRet, c.loopDepth, c.fnDepth = savedRet, savedLoop, savedDepth
	return sig
}

func describeFn(name string) string {
	if name == "" {
		return "this function"
	}
	return "function " + `"` + name + `"`
}

// containsReturn reports whether a body returns explicitly. It deliberately
// does not look inside nested functions.
func containsReturn(stmt Statement) bool {
	switch node := stmt.(type) {
	case *ReturnStatement:
		return true
	case *BlockStatement:
		for _, s := range node.Statements {
			if containsReturn(s) {
				return true
			}
		}
	case *IfStatement:
		if containsReturn(node.Consequence) {
			return true
		}
		if node.Alternative != nil && containsReturn(node.Alternative) {
			return true
		}
	case *ForStatement:
		return containsReturn(node.Body)
	case *WhileStatement:
		return containsReturn(node.Body)
	}
	return false
}

func (c *Checker) checkBlock(block *BlockStatement) *Type {
	c.pushScope()
	t := c.checkBlockNoScope(block)
	c.popScope()
	return t
}

func (c *Checker) checkBlockNoScope(block *BlockStatement) *Type {
	var last *Type = nilT
	for _, stmt := range block.Statements {
		last = c.checkStatement(stmt)
	}
	return last
}

// ---------------------------------------------------------------- expressions

func (c *Checker) checkExpression(expr Expression) *Type {
	switch node := expr.(type) {

	case *IntegerLiteral:
		return intT
	case *FloatLiteral:
		return floatT
	case *BooleanLiteral:
		return boolT
	case *NilLiteral:
		return nilT
	case *StringLiteral:
		return c.checkStringLiteral(node)

	case *Identifier:
		if t, ok := c.lookup(node.Value); ok {
			return t
		}
		if _, ok := c.builtins[node.Value]; ok {
			return anyT
		}
		c.errorf(node.Token, "undefined variable %q", node.Value)
		return anyT

	case *PrefixExpression:
		return c.checkPrefix(node)

	case *InfixExpression:
		return c.checkBinary(node, c.checkExpression(node.Left), c.checkExpression(node.Right))

	case *AssignExpression:
		return c.checkAssign(node)

	case *CallExpression:
		return c.checkCall(node)

	case *IndexExpression:
		return c.checkIndex(node)

	case *ArrayLiteral:
		if len(node.Elements) == 0 {
			return arrayOf(anyT)
		}
		elem := c.checkExpression(node.Elements[0])
		for _, el := range node.Elements[1:] {
			elem = unify(elem, c.checkExpression(el))
		}
		return arrayOf(elem)

	case *FunctionLiteral:
		return c.checkFunctionBody("", node.Parameters, node.ReturnType, node.Body, node.Token)

	case *MemberExpression:
		// Property access is not implemented yet; the interpreter reports it.
		return anyT
	}

	return anyT
}

// checkStringLiteral also type-checks the expressions embedded in a string.
func (c *Checker) checkStringLiteral(node *StringLiteral) *Type {
	if !strings.Contains(node.Value, "{") {
		return strT
	}

	segs, err := splitInterpolation(node.Value)
	if err != nil {
		c.errorf(node.Token, "%s", err)
		return strT
	}

	for _, seg := range segs {
		if seg.Expr == "" {
			continue
		}
		p := NewParser(NewLexerAt(seg.Expr, node.Token.Line))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			c.errorf(node.Token, "bad interpolation {%s}: %s", seg.Expr, strings.Join(p.Errors(), "; "))
			continue
		}
		if len(program.Statements) != 1 {
			c.errorf(node.Token, "interpolation {%s} must be one expression", seg.Expr)
			continue
		}
		stmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			c.errorf(node.Token, "interpolation {%s} must be an expression", seg.Expr)
			continue
		}
		c.checkExpression(stmt.Expression)
	}

	return strT
}

func (c *Checker) checkPrefix(node *PrefixExpression) *Type {
	rightType := c.checkExpression(node.Right)

	switch node.Operator {
	case "!":
		if !isBoolish(rightType) {
			c.errorf(node.Token, "'!' expects a bool, got %s", rightType)
		}
		return boolT
	case "-":
		switch rightType.Kind {
		case TypeInt:
			return intT
		case TypeFloat:
			return floatT
		case TypeAny:
			return anyT
		}
		c.errorf(node.Token, "'-' expects a number, got %s", rightType)
		return anyT
	}

	return anyT
}

func (c *Checker) checkBinary(node *InfixExpression, leftType, rightType *Type) *Type {
	op := node.Operator

	switch op {
	case "&&", "||":
		if !isBoolish(leftType) {
			c.errorf(node.Token, "'%s' expects bool operands, got %s", op, leftType)
		}
		if !isBoolish(rightType) {
			c.errorf(node.Token, "'%s' expects bool operands, got %s", op, rightType)
		}
		return boolT

	case "==", "!=":
		return boolT

	case "<", ">", "<=", ">=":
		if leftType.Kind == TypeAny || rightType.Kind == TypeAny {
			return boolT
		}
		if leftType.Kind != rightType.Kind {
			c.errorf(node.Token, "cannot compare %s and %s with '%s'", leftType, rightType, op)
			return boolT
		}
		if !isNumericKind(leftType.Kind) && leftType.Kind != TypeStr {
			c.errorf(node.Token, "cannot compare %s values with '%s'", leftType, op)
		}
		return boolT

	case "+":
		if leftType.Kind == TypeAny || rightType.Kind == TypeAny {
			return anyT
		}
		if leftType.Kind != rightType.Kind {
			if isNumericKind(leftType.Kind) && isNumericKind(rightType.Kind) {
				c.errorf(node.Token, "cannot add %s and %s; convert one explicitly with int() or float()", leftType, rightType)
			} else {
				c.errorf(node.Token, "cannot add %s and %s", leftType, rightType)
			}
			return anyT
		}
		switch leftType.Kind {
		case TypeInt:
			return intT
		case TypeFloat:
			return floatT
		case TypeStr:
			return strT
		case TypeArray:
			return arrayOf(unify(leftType.Elem, rightType.Elem))
		}
		c.errorf(node.Token, "cannot add %s and %s", leftType, rightType)
		return anyT

	case "-", "*", "/", "%":
		if leftType.Kind == TypeAny || rightType.Kind == TypeAny {
			return anyT
		}
		if leftType.Kind != rightType.Kind {
			if isNumericKind(leftType.Kind) && isNumericKind(rightType.Kind) {
				c.errorf(node.Token, "cannot use '%s' on %s and %s; convert one explicitly with int() or float()", op, leftType, rightType)
			} else {
				c.errorf(node.Token, "cannot use '%s' on %s and %s", op, leftType, rightType)
			}
			return anyT
		}
		switch leftType.Kind {
		case TypeInt:
			return intT
		case TypeFloat:
			return floatT
		}
		c.errorf(node.Token, "cannot use '%s' on %s and %s", op, leftType, rightType)
		return anyT
	}

	return anyT
}

func (c *Checker) checkAssign(node *AssignExpression) *Type {
	valueType := c.checkExpression(node.Value)

	switch target := node.Target.(type) {
	case *Identifier:
		existing, ok := c.lookup(target.Value)
		if !ok {
			c.errorf(node.Token, "undefined variable %q; use 'let' to declare it first", target.Value)
			return valueType
		}
		if !assignable(existing, valueType) {
			c.errorf(node.Token, "cannot assign %s to %s variable %q", valueType, existing, target.Value)
		}
		return valueType

	case *IndexExpression:
		containerType := c.checkExpression(target.Left)
		indexType := c.checkExpression(target.Index)
		if indexType.Kind != TypeAny && indexType.Kind != TypeInt {
			c.errorf(node.Token, "index must be an int, got %s", indexType)
		}
		switch containerType.Kind {
		case TypeArray:
			if !assignable(containerType.Elem, valueType) {
				c.errorf(node.Token, "cannot assign %s into array of %s", valueType, containerType.Elem)
			}
		case TypeAny:
			// unknown container, nothing to check
		default:
			c.errorf(node.Token, "cannot assign into %s", containerType)
		}
		return valueType
	}

	c.errorf(node.Token, "cannot assign to this expression")
	return valueType
}

func (c *Checker) checkIndex(node *IndexExpression) *Type {
	containerType := c.checkExpression(node.Left)
	indexType := c.checkExpression(node.Index)

	if indexType.Kind != TypeAny && indexType.Kind != TypeInt {
		c.errorf(node.Token, "index must be an int, got %s", indexType)
	}

	switch containerType.Kind {
	case TypeArray:
		if containerType.Elem == nil {
			return anyT
		}
		return containerType.Elem
	case TypeStr:
		return strT
	case TypeAny:
		return anyT
	}

	c.errorf(node.Token, "cannot index %s", containerType)
	return anyT
}

func (c *Checker) checkCall(node *CallExpression) *Type {
	label := "this value"
	if id, ok := node.Function.(*Identifier); ok {
		label = id.Value + "()"

		// Built-ins get precise per-function checks.
		if sig, found := c.builtins[id.Value]; found {
			argTypes := c.checkArgs(node.Arguments)
			return sig.Check(c, argTypes, node.Token)
		}
	}

	calleeType := c.checkExpression(node.Function)
	argTypes := c.checkArgs(node.Arguments)

	if calleeType.Kind == TypeAny {
		return anyT
	}
	if calleeType.Kind != TypeFn {
		c.errorf(node.Token, "%s is not callable", calleeType)
		return anyT
	}

	if calleeType.Variadic {
		if len(argTypes) < len(calleeType.Params)-1 {
			c.errorf(node.Token, "%s expects at least %d argument(s), got %d", label, len(calleeType.Params)-1, len(argTypes))
		}
		return calleeType.Ret
	}

	if len(argTypes) != len(calleeType.Params) {
		c.errorf(node.Token, "%s expects %d argument(s), got %d", label, len(calleeType.Params), len(argTypes))
		return calleeType.Ret
	}

	for i, argType := range argTypes {
		if !assignable(calleeType.Params[i], argType) {
			c.errorf(node.Token, "argument %d: cannot use %s where %s is expected", i+1, argType, calleeType.Params[i])
		}
	}

	return calleeType.Ret
}

func (c *Checker) checkArgs(args []Expression) []*Type {
	types := make([]*Type, 0, len(args))
	for _, a := range args {
		types = append(types, c.checkExpression(a))
	}
	return types
}

// ---------------------------------------------------------------- built-ins

func (c *Checker) expectArgs(name string, args []*Type, want int, tok Token) bool {
	if len(args) != want {
		c.errorf(tok, "%s() takes %d argument(s), got %d", name, want, len(args))
		return false
	}
	return true
}

// newBuiltinSigs describes every built-in so calls can be checked precisely.
func newBuiltinSigs() map[string]*builtinSig {
	sigs := map[string]*builtinSig{}

	sigs["print"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		return nilT
	}}

	sigs["len"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("len", args, 1, tok) {
			return intT
		}
		switch args[0].Kind {
		case TypeStr, TypeArray, TypeAny:
			return intT
		}
		c.errorf(tok, "len() expects str or array, got %s", args[0])
		return intT
	}}

	sigs["push"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("push", args, 2, tok) {
			return arrayOf(anyT)
		}
		switch args[0].Kind {
		case TypeArray:
			return arrayOf(unify(args[0].Elem, args[1]))
		case TypeAny:
			return arrayOf(anyT)
		}
		c.errorf(tok, "push() expects array, got %s", args[0])
		return arrayOf(anyT)
	}}

	sigs["str"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		c.expectArgs("str", args, 1, tok)
		return strT
	}}

	sigs["int"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("int", args, 1, tok) {
			return intT
		}
		switch args[0].Kind {
		case TypeInt, TypeFloat, TypeBool, TypeStr, TypeAny:
			return intT
		}
		c.errorf(tok, "int() expects int, float, bool or str, got %s", args[0])
		return intT
	}}

	sigs["float"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("float", args, 1, tok) {
			return floatT
		}
		switch args[0].Kind {
		case TypeInt, TypeFloat, TypeStr, TypeAny:
			return floatT
		}
		c.errorf(tok, "float() expects int, float or str, got %s", args[0])
		return floatT
	}}

	sigs["type"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		c.expectArgs("type", args, 1, tok)
		return strT
	}}

	sigs["range"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if len(args) != 1 && len(args) != 2 {
			c.errorf(tok, "range() takes 1 or 2 argument(s), got %d", len(args))
			return arrayOf(intT)
		}
		for _, a := range args {
			if a.Kind != TypeInt && a.Kind != TypeAny {
				c.errorf(tok, "range() expects int, got %s", a)
			}
		}
		return arrayOf(intT)
	}}

	for _, name := range []string{"upper", "lower", "trim"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if !c.expectArgs(n, args, 1, tok) {
				return strT
			}
			if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
				c.errorf(tok, "%s() expects str, got %s", n, args[0])
			}
			return strT
		}}
	}

	sigs["split"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("split", args, 2, tok) {
			return arrayOf(strT)
		}
		for _, a := range args {
			if a.Kind != TypeStr && a.Kind != TypeAny {
				c.errorf(tok, "split() expects str, got %s", a)
			}
		}
		return arrayOf(strT)
	}}

	sigs["join"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("join", args, 2, tok) {
			return strT
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "join() expects array, got %s", args[0])
		}
		if args[1].Kind != TypeStr && args[1].Kind != TypeAny {
			c.errorf(tok, "join() expects str as the separator, got %s", args[1])
		}
		return strT
	}}

	sigs["contains"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("contains", args, 2, tok) {
			return boolT
		}
		switch args[0].Kind {
		case TypeStr, TypeArray, TypeAny:
		default:
			c.errorf(tok, "contains() expects str or array, got %s", args[0])
		}
		return boolT
	}}

	sigs["abs"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("abs", args, 1, tok) {
			return anyT
		}
		switch args[0].Kind {
		case TypeInt:
			return intT
		case TypeFloat:
			return floatT
		case TypeAny:
			return anyT
		}
		c.errorf(tok, "abs() expects int or float, got %s", args[0])
		return anyT
	}}

	for _, name := range []string{"min", "max"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if len(args) == 0 {
				c.errorf(tok, "%s() needs at least one argument", n)
				return anyT
			}
			result := args[0]
			for _, a := range args {
				if a.Kind != TypeInt && a.Kind != TypeFloat && a.Kind != TypeAny {
					c.errorf(tok, "%s() expects numbers, got %s", n, a)
					return anyT
				}
				result = unify(result, a)
			}
			return result
		}}
	}

	return sigs
}
