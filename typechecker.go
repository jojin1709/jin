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
	TypeMap
	TypeFn
	// TypeStruct is an instance of a declared record type, and TypeStructType
	// is the type name itself, which is a value: `Point` in `Point(1, 2)`.
	TypeStruct
	TypeStructType
)

// StructInfo is one record type's shape: the name it was declared under and its
// fields, in the order they were written. The name and every value of that type
// point at the same info, so a field may name a struct written further down the
// file — including the struct it is itself a field of — without the shape
// having to be finished when the field is read.
type StructInfo struct {
	Name    string
	Fields  []StructField
	Methods map[string]*Type
}

// StructField is one field of a record type.
type StructField struct {
	Name string
	Type *Type
}

// Type describes the type of a value.
type Type struct {
	Kind     TypeKind
	Elem     *Type   // arrays and maps
	Params   []*Type // functions
	Ret      *Type   // functions
	Variadic bool
	Struct   *StructInfo // record types
}

var (
	anyT   = &Type{Kind: TypeAny}
	intT   = &Type{Kind: TypeInt}
	floatT = &Type{Kind: TypeFloat}
	strT   = &Type{Kind: TypeStr}
	boolT  = &Type{Kind: TypeBool}
	nilT   = &Type{Kind: TypeNil}
	mapT   = &Type{Kind: TypeMap}
)

func arrayOf(elem *Type) *Type { return &Type{Kind: TypeArray, Elem: elem} }

// structTypeValue is the type of a struct's name: the value `Point`, which can
// be called to build an instance and can be passed around like any other value.
func structTypeValue(info *StructInfo) *Type {
	return &Type{Kind: TypeStructType, Struct: info}
}

// structInstance is the type of an instance of a record, the type `Point(1, 2)`
// has.
func structInstance(info *StructInfo) *Type {
	return &Type{Kind: TypeStruct, Struct: info}
}

// structName reports the name a record type was declared under. Only the two
// record kinds have one.
func structName(t *Type) string {
	if t.Struct == nil {
		return ""
	}
	return t.Struct.Name
}

// fieldOf finds one of a record's fields by name.
func fieldOf(t *Type, name string) (StructField, bool) {
	if t.Struct == nil {
		return StructField{}, false
	}
	for _, f := range t.Struct.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return StructField{}, false
}

// methodOf finds one of a record's methods by name.
func methodOf(t *Type, name string) (*Type, bool) {
	if t.Struct == nil || t.Struct.Methods == nil {
		return nil, false
	}
	m, ok := t.Struct.Methods[name]
	return m, ok
}

// mapOf describes a map. Jin has no record types, so a map is how a value with
// named fields is written; that means its values are free to differ in type and
// the element type is only recorded when there is something precise to say.
func mapOf(elem *Type) *Type { return &Type{Kind: TypeMap, Elem: elem} }

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
	case TypeMap:
		if t.Elem == nil || t.Elem.Kind == TypeAny {
			return "map"
		}
		return "map<" + t.Elem.String() + ">"
	case TypeFn:
		parts := make([]string, 0, len(t.Params))
		for _, p := range t.Params {
			parts = append(parts, p.String())
		}
		if t.Variadic && len(parts) > 0 {
			parts[len(parts)-1] += "..."
		}
		return "fn(" + strings.Join(parts, ", ") + ") -> " + t.Ret.String()
	case TypeStruct, TypeStructType:
		if t.Struct == nil {
			return "?"
		}
		return t.Struct.Name
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
	case TypeArray, TypeMap:
		return assignable(target.Elem, value.Elem)
	case TypeStruct, TypeStructType:
		// Records are matched by name: the name is what the value carries at
		// run time, and one record type is not another even when their fields
		// happen to line up.
		return structName(target) == structName(value)
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
	if a.Kind == TypeStruct || a.Kind == TypeStructType {
		// Two different record types have no common type but `any`, unless
		// they are the same record, in which case that record is the answer.
		if structName(a) != structName(b) {
			return anyT
		}
		return a
	}
	if a.Kind == TypeArray {
		return arrayOf(unify(a.Elem, b.Elem))
	}
	if a.Kind == TypeMap {
		return mapOf(unify(a.Elem, b.Elem))
	}
	return a
}

func isBoolish(t *Type) bool { return t.Kind == TypeBool || t.Kind == TypeAny }

func isNumericKind(k TypeKind) bool { return k == TypeInt || k == TypeFloat }

// ---------------------------------------------------------------- checker

// checkError is one problem found by the checker, with its position.
type checkError struct {
	File string
	Line int
	Col  int
	Msg  string
}

// Checker walks a syntax tree and reports type errors before the program runs.
type Checker struct {
	errors   []checkError
	scopes   []map[string]*Type
	builtins map[string]*builtinSig

	// The names this file can reach other files by, and the top-level types of
	// every file checked so far.
	aliases map[string]*Module
	modules map[*Module]map[string]*Type

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
		aliases:  map[string]*Module{},
		modules:  map[*Module]map[string]*Type{},
	}
}

// Check type-checks a whole program and returns every error it found, sorted
// by position so a file's problems read top to bottom.
func Check(program *Program) []string {
	return NewChecker().CheckProgram(program)
}

// CheckProject type-checks a file and everything it imports, each file on its
// own but with the files it imports already known. Errors come out grouped by
// file, in the order the files run.
func CheckProject(entry *Module) []string {
	return NewChecker().CheckModules(entry, map[*Module]map[string]*Type{})
}

// CheckModules checks every imported file that has not been checked yet, then
// the file itself. Each imported file is checked on its own so its names stay
// its own, and its top-level types are recorded in checked for the files that
// import it. A REPL passes the same checked map every line, so a file imported
// on one line is still known on the next.
func (c *Checker) CheckModules(entry *Module, checked map[*Module]map[string]*Type) []string {
	var out []string

	for _, m := range entry.Deps {
		if _, done := checked[m]; done {
			continue
		}
		sub := NewChecker()
		sub.aliases = m.Aliases()
		sub.modules = checked
		out = append(out, sub.CheckProgram(m.Program)...)
		checked[m] = sub.GlobalScope()
	}

	for alias, mod := range entry.Aliases() {
		c.aliases[alias] = mod
	}
	c.modules = checked
	out = append(out, c.CheckProgram(entry.Program)...)
	return out
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
		if fn.Receiver != nil {
			methodParams := append([]*Parameter{fn.Receiver}, fn.Parameters...)
			c.checkFunctionBody(fn.Receiver.Type.Name+"."+fn.Name.Value, methodParams, fn.ReturnType, fn.Body, fn.Token)
		} else {
			c.checkFunctionBody(fn.Name.Value, fn.Parameters, fn.ReturnType, fn.Body, fn.Token)
		}
	}

	sort.SliceStable(c.errors, func(i, j int) bool {
		if c.errors[i].Line != c.errors[j].Line {
			return c.errors[i].Line < c.errors[j].Line
		}
		return c.errors[i].Col < c.errors[j].Col
	})

	out := make([]string, 0, len(c.errors))
	for _, e := range c.errors {
		out = append(out, fmt.Sprintf("%s: %s", checkPosition(e), e.Msg))
	}
	return out
}

// checkPosition writes where an error is, naming the file only when the code
// came from an imported one.
func checkPosition(e checkError) string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d", e.File, e.Line, e.Col)
	}
	return fmt.Sprintf("%d:%d", e.Line, e.Col)
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
		File: tok.File,
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

// predeclare registers every top-level function so calls can go in any order,
// and every record type, whose fields are resolved in a second pass.
func (c *Checker) predeclare(program *Program) {
	// Record names go in first, all of them, so a field may name a struct
	// written further down the file — including the struct it is a field of,
	// which is how a linked node is written.
	type declared struct {
		info *StructInfo
		node *StructStatement
	}
	var records []declared
	for _, stmt := range program.Statements {
		if node, ok := stmt.(*StructStatement); ok {
			info := &StructInfo{Name: node.Name.Value}
			c.declare(node.Name.Value, structTypeValue(info))
			records = append(records, declared{info: info, node: node})
		}
	}

	// Only now are the field types read, so every name they might use is
	// already known.
	for _, rec := range records {
		seen := map[string]bool{}
		for _, f := range rec.node.Fields {
			if seen[f.Name.Value] {
				c.errorf(f.Token, "duplicate field %q in struct %s", f.Name.Value, rec.info.Name)
				continue
			}
			seen[f.Name.Value] = true
			rec.info.Fields = append(rec.info.Fields, StructField{
				Name: f.Name.Value,
				Type: c.annotationType(f.Type),
			})
		}
	}

	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*FunctionStatement); ok {
			if fn.Receiver != nil {
				recType, ok := c.lookup(fn.Receiver.Type.Name)
				if !ok || recType.Kind != TypeStructType {
					c.errorf(fn.Receiver.Token, "cannot define method on unknown type %q", fn.Receiver.Type.Name)
					continue
				}
				info := recType.Struct
				if _, hasField := fieldOf(structInstance(info), fn.Name.Value); hasField {
					c.errorf(fn.Name.Token, "method %q clashes with field of struct %s", fn.Name.Value, info.Name)
					continue
				}
				if info.Methods == nil {
					info.Methods = make(map[string]*Type)
				}
				if _, hasMethod := info.Methods[fn.Name.Value]; hasMethod {
					c.errorf(fn.Name.Token, "duplicate method %q on struct %s", fn.Name.Value, info.Name)
					continue
				}
				info.Methods[fn.Name.Value] = c.fnSignature(fn.Parameters, fn.ReturnType)
			} else {
				c.declare(fn.Name.Value, c.fnSignature(fn.Parameters, fn.ReturnType))
			}
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
	case "map":
		// A map's values may differ in type, so "map" means map<any>.
		return mapOf(anyT)
	case "any":
		return anyT
	}

	// A type name that is not built in may be a record declared in this file,
	// which is what makes `let p: Point = ...` and a field of type Point work.
	if t, ok := c.lookup(ann.Name); ok && t.Kind == TypeStructType {
		return structInstance(t.Struct)
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

	case *ImportStatement:
		// The file it names was checked before this one, and its top-level
		// types are in c.modules, so there is nothing to check here.
		return nilT

	case *StructStatement:
		// A record type is declared, and its field types are resolved, in
		// predeclare. There is nothing left to check by the time its statement
		// is reached.
		return nilT

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
		case TypeMap:
			// A map loops over its keys, which may be an int, a str or a bool.
			elemType = anyT
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

	case *TryStatement:
		tryType := c.checkBlock(node.TryBody)
		if _, isBuiltin := c.builtins[node.CatchIdent.Value]; isBuiltin {
			c.errorf(node.CatchIdent.Token, "catch variable %q cannot shadow a builtin", node.CatchIdent.Value)
		}
		c.pushScope()
		c.declare(node.CatchIdent.Value, strT)
		catchType := c.checkBlockNoScope(node.CatchBody)
		c.popScope()
		return unify(tryType, catchType)

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
	case *TryStatement:
		if containsReturn(node.TryBody) {
			return true
		}
		if containsReturn(node.CatchBody) {
			return true
		}
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
	// Functions in a block are declared before any body is checked, so two of
	// them can call each other in either order. This mirrors the compiler,
	// which predefines them too.
	for _, stmt := range block.Statements {
		if fn, ok := stmt.(*FunctionStatement); ok {
			c.declare(fn.Name.Value, c.fnSignature(fn.Parameters, fn.ReturnType))
		}
	}

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
		if _, isModule := c.aliases[node.Value]; isModule {
			c.errorf(node.Token, "%q is a module; write %s.<name> to use a name from it",
				node.Value, node.Value)
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

	case *MapLiteral:
		// Every key and value is still checked, but the map itself is
		// deliberately loose: a map holding a name and an age at once is the
		// normal way to write a record here.
		for _, pair := range node.Pairs {
			c.checkExpression(pair.Key)
			c.checkExpression(pair.Value)
		}
		return mapOf(anyT)

	case *FunctionLiteral:
		return c.checkFunctionBody("", node.Parameters, node.ReturnType, node.Body, node.Token)

	case *MemberExpression:
		return c.checkMember(node)

	case *StructLiteral:
		return c.checkStructLiteral(node)
	}

	return anyT
}

func (c *Checker) checkStructLiteral(node *StructLiteral) *Type {
	t := c.checkExpression(node.Struct)
	if t.Kind != TypeStructType {
		c.errorf(node.Token, "%s is not a record type", t)
		return anyT
	}
	info := t.Struct
	if info == nil {
		return anyT
	}

	seen := make(map[string]bool)
	for _, f := range node.Fields {
		if seen[f.Name.Value] {
			c.errorf(f.Name.Token, "duplicate field %q in %s", f.Name.Value, info.Name)
		}
		seen[f.Name.Value] = true

		fieldInfo, found := fieldOf(t, f.Name.Value)
		if !found {
			c.errorf(f.Name.Token, "%s has no field %q", info.Name, f.Name.Value)
			c.checkExpression(f.Value)
			continue
		}

		valType := c.checkExpression(f.Value)
		if !assignable(fieldInfo.Type, valType) {
			c.errorf(f.Name.Token, "field %q: cannot use %s where %s is expected", f.Name.Value, valType, fieldInfo.Type)
		}
	}

	for _, f := range info.Fields {
		if !seen[f.Name] {
			c.errorf(node.Token, "missing field %q in %s", f.Name, info.Name)
		}
	}

	return structInstance(info)
}

// checkMember reads a name from an imported file (u.name), or a field of a
// record (p.x). The module case is settled first, because an alias is not a
// value; everything else is asked what it is, and only a record has fields.
func (c *Checker) checkMember(node *MemberExpression) *Type {
	if id, ok := node.Object.(*Identifier); ok {
		if _, shadowed := c.lookup(id.Value); !shadowed {
			if mod, isModule := c.aliases[id.Value]; isModule {
				if t, ok := c.modules[mod][node.Property.Value]; ok {
					return t
				}
				c.errorf(node.Token, "%s has no name %q", id.Value, node.Property.Value)
				return anyT
			}
		}
	}

	objType := c.checkExpression(node.Object)
	switch objType.Kind {
	case TypeStruct:
		if f, ok := fieldOf(objType, node.Property.Value); ok {
			return f.Type
		}
		if m, ok := methodOf(objType, node.Property.Value); ok {
			return m
		}
		c.errorf(node.Token, "%s has no field %q", objType, node.Property.Value)
		return anyT
	case TypeStructType:
		c.errorf(node.Token, "cannot read a field of the type %s; make a %s first", objType, objType)
		return anyT
	case TypeAny:
		// The value's shape is unknown, so the run time has the last word.
		return anyT
	case TypeMap:
		c.errorf(node.Token, `cannot read a field of map; use m[%q] to read a key`, node.Property.Value)
		return anyT
	}
	c.errorf(node.Token, "cannot read a field of %s", objType)
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
		p := NewParser(NewLexerAt(seg.Expr, node.Token.Line, node.Token.File))
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
		switch containerType.Kind {
		case TypeMap:
			c.checkMapKey(node.Token, indexType)
			return valueType
		case TypeAny:
			// unknown container, nothing to check
			return valueType
		}
		if indexType.Kind != TypeAny && indexType.Kind != TypeInt {
			c.errorf(node.Token, "index must be an int, got %s", indexType)
		}
		switch containerType.Kind {
		case TypeArray:
			if !assignable(containerType.Elem, valueType) {
				c.errorf(node.Token, "cannot assign %s into array of %s", valueType, containerType.Elem)
			}
		default:
			c.errorf(node.Token, "cannot assign into %s", containerType)
		}
		return valueType

	case *MemberExpression:
		if id, ok := target.Object.(*Identifier); ok {
			if _, shadowed := c.lookup(id.Value); !shadowed {
				if _, isModule := c.aliases[id.Value]; isModule {
					c.errorf(target.Token, "cannot assign to %s.%s; another file's names are read-only",
						id.Value, target.Property.Value)
					return valueType
				}
			}
		}

		objType := c.checkExpression(target.Object)
		switch objType.Kind {
		case TypeStruct:
			f, ok := fieldOf(objType, target.Property.Value)
			if !ok {
				if _, isMethod := methodOf(objType, target.Property.Value); isMethod {
					c.errorf(target.Token, "cannot assign to method %q of %s", target.Property.Value, objType)
					return valueType
				}
				c.errorf(target.Token, "%s has no field %q", objType, target.Property.Value)
				return valueType
			}
			if !assignable(f.Type, valueType) {
				c.errorf(target.Token, "cannot assign %s to %s field %q", valueType, f.Type, target.Property.Value)
			}
			return valueType
		case TypeStructType:
			c.errorf(target.Token, "cannot assign a field of the type %s; make a %s first", objType, objType)
			return valueType
		case TypeAny:
			// The value's shape is unknown, so the run time has the last word.
			return valueType
		}
		c.errorf(target.Token, "cannot assign a field of %s", objType)
		return valueType
	}

	c.errorf(node.Token, "cannot assign to this expression")
	return valueType
}

func (c *Checker) checkIndex(node *IndexExpression) *Type {
	containerType := c.checkExpression(node.Left)
	indexType := c.checkExpression(node.Index)

	switch containerType.Kind {
	case TypeMap:
		// A map is looked up by key, and a key may be an int, a str or a bool.
		c.checkMapKey(node.Token, indexType)
		if containerType.Elem == nil {
			return anyT
		}
		return containerType.Elem

	case TypeArray, TypeStr:
		if indexType.Kind != TypeAny && indexType.Kind != TypeInt {
			c.errorf(node.Token, "index must be an int, got %s", indexType)
		}
		if containerType.Kind == TypeStr {
			return strT
		}
		if containerType.Elem == nil {
			return anyT
		}
		return containerType.Elem

	case TypeAny:
		// The value could be an array, a string or a map, so any key fits.
		return anyT
	}

	c.errorf(node.Token, "cannot index %s", containerType)
	return anyT
}

// checkMapKey makes sure a key is one of the values a map can be keyed by.
func (c *Checker) checkMapKey(tok Token, keyType *Type) {
	switch keyType.Kind {
	case TypeInt, TypeStr, TypeBool, TypeAny:
		return
	}
	c.errorf(tok, "map keys must be int, str or bool, got %s", keyType)
}

func (c *Checker) checkCall(node *CallExpression) *Type {
	label := "this value"
	if id, ok := node.Function.(*Identifier); ok {
		label = id.Value + "()"

		// Built-ins get precise per-function checks, but a local or global with
		// the same name hides the built-in, just as it does at run time.
		if _, shadowed := c.lookup(id.Value); !shadowed {
			if sig, found := c.builtins[id.Value]; found {
				argTypes := c.checkArgs(node.Arguments)
				return sig.Check(c, argTypes, node.Token)
			}
		}
	}
	if member, ok := node.Function.(*MemberExpression); ok {
		if id, ok := member.Object.(*Identifier); ok {
			label = id.Value + "." + member.Property.Value + "()"
		}
	}

	calleeType := c.checkExpression(node.Function)
	argTypes := c.checkArgs(node.Arguments)

	// A record type's name is a constructor: Point(1, 2) makes an instance
	// whose fields are the values, in the order the struct declared them.
	if calleeType.Kind == TypeStructType {
		return c.checkConstructor(calleeType, argTypes, node.Token)
	}

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

// checkConstructor checks a call of a record type's name: one value per field,
// in the order the fields were declared, each of them the right type.
func (c *Checker) checkConstructor(t *Type, argTypes []*Type, tok Token) *Type {
	info := t.Struct
	instance := structInstance(info)
	if len(argTypes) != len(info.Fields) {
		c.errorf(tok, "%s(...) expects %d value(s), got %d", t, len(info.Fields), len(argTypes))
		return instance
	}
	for i, f := range info.Fields {
		if !assignable(f.Type, argTypes[i]) {
			c.errorf(tok, "field %q: cannot use %s where %s is expected", f.Name, argTypes[i], f.Type)
		}
	}
	return instance
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

	sigs["args"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		c.expectArgs("args", args, 0, tok)
		return arrayOf(strT)
	}}

	sigs["len"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("len", args, 1, tok) {
			return intT
		}
		switch args[0].Kind {
		case TypeStr, TypeArray, TypeMap, TypeAny:
			return intT
		}
		c.errorf(tok, "len() expects str, array or map, got %s", args[0])
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
		case TypeMap:
			c.checkMapKey(tok, args[1])
		default:
			c.errorf(tok, "contains() expects str, array or map, got %s", args[0])
		}
		return boolT
	}}

	sigs["has"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("has", args, 2, tok) {
			return boolT
		}
		if args[0].Kind != TypeMap && args[0].Kind != TypeAny {
			c.errorf(tok, "has() expects map, got %s", args[0])
			return boolT
		}
		c.checkMapKey(tok, args[1])
		return boolT
	}}

	sigs["get"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("get", args, 3, tok) {
			return anyT
		}
		if args[0].Kind != TypeMap && args[0].Kind != TypeAny {
			c.errorf(tok, "get() expects map, got %s", args[0])
			return anyT
		}
		c.checkMapKey(tok, args[1])
		return unify(args[0].Elem, args[2])
	}}

	sigs["delete"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("delete", args, 2, tok) {
			return nilT
		}
		if args[0].Kind != TypeMap && args[0].Kind != TypeAny {
			c.errorf(tok, "delete() expects map, got %s", args[0])
			return nilT
		}
		c.checkMapKey(tok, args[1])
		return nilT
	}}

	sigs["keys"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("keys", args, 1, tok) {
			return arrayOf(anyT)
		}
		if args[0].Kind != TypeMap && args[0].Kind != TypeAny {
			c.errorf(tok, "keys() expects map, got %s", args[0])
		}
		return arrayOf(anyT)
	}}

	sigs["values"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("values", args, 1, tok) {
			return arrayOf(anyT)
		}
		if args[0].Kind != TypeMap && args[0].Kind != TypeAny {
			c.errorf(tok, "values() expects map, got %s", args[0])
		}
		if args[0].Kind == TypeMap || args[0].Kind == TypeAny {
			return arrayOf(args[0].Elem)
		}
		return arrayOf(anyT)
	}}

	// ------------------------------------------------------------- files

	for _, name := range []string{"read_file"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if !c.expectArgs(n, args, 1, tok) {
				return strT
			}
			if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
				c.errorf(tok, "%s() expects a path as str, got %s", n, args[0])
			}
			return strT
		}}
	}

	for _, name := range []string{"write_file", "append_file"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if !c.expectArgs(n, args, 2, tok) {
				return nilT
			}
			for i, a := range args {
				if a.Kind != TypeStr && a.Kind != TypeAny {
					c.errorf(tok, "%s() argument %d expects str, got %s", n, i+1, a)
				}
			}
			return nilT
		}}
	}

	sigs["file_exists"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("file_exists", args, 1, tok) {
			return boolT
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "file_exists() expects a path as str, got %s", args[0])
		}
		return boolT
	}}

	sigs["delete_file"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("delete_file", args, 1, tok) {
			return nilT
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "delete_file() expects a path as str, got %s", args[0])
		}
		return nilT
	}}

	// ------------------------------------------------------------- collections

	sigs["sort"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("sort", args, 1, tok) {
			return arrayOf(anyT)
		}
		switch args[0].Kind {
		case TypeArray:
			return args[0]
		case TypeAny:
			return arrayOf(anyT)
		}
		c.errorf(tok, "sort() expects array, got %s", args[0])
		return arrayOf(anyT)
	}}

	sigs["reverse"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("reverse", args, 1, tok) {
			return anyT
		}
		switch args[0].Kind {
		case TypeArray, TypeStr:
			return args[0]
		case TypeAny:
			return anyT
		}
		c.errorf(tok, "reverse() expects array or str, got %s", args[0])
		return anyT
	}}

	sigs["slice"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("slice", args, 3, tok) {
			return anyT
		}
		for _, i := range []int{1, 2} {
			if args[i].Kind != TypeInt && args[i].Kind != TypeAny {
				c.errorf(tok, "slice() expects int bounds, got %s", args[i])
			}
		}
		switch args[0].Kind {
		case TypeArray, TypeStr:
			return args[0]
		case TypeAny:
			return anyT
		}
		c.errorf(tok, "slice() expects array or str, got %s", args[0])
		return anyT
	}}

	// ------------------------------------------------------------- strings

	sigs["replace"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("replace", args, 3, tok) {
			return strT
		}
		for i, a := range args {
			if a.Kind != TypeStr && a.Kind != TypeAny {
				c.errorf(tok, "replace() argument %d expects str, got %s", i+1, a)
			}
		}
		return strT
	}}

	sigs["find"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("find", args, 2, tok) {
			return intT
		}
		for i, a := range args {
			if a.Kind != TypeStr && a.Kind != TypeAny {
				c.errorf(tok, "find() argument %d expects str, got %s", i+1, a)
			}
		}
		return intT
	}}

	for _, name := range []string{"starts_with", "ends_with"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if !c.expectArgs(n, args, 2, tok) {
				return boolT
			}
			for i, a := range args {
				if a.Kind != TypeStr && a.Kind != TypeAny {
					c.errorf(tok, "%s() argument %d expects str, got %s", n, i+1, a)
				}
			}
			return boolT
		}}
	}

	sigs["repeat"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("repeat", args, 2, tok) {
			return strT
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "repeat() expects str, got %s", args[0])
		}
		if args[1].Kind != TypeInt && args[1].Kind != TypeAny {
			c.errorf(tok, "repeat() expects int, got %s", args[1])
		}
		return strT
	}}

	sigs["chars"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("chars", args, 1, tok) {
			return arrayOf(strT)
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "chars() expects str, got %s", args[0])
		}
		return arrayOf(strT)
	}}

	sigs["lines"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("lines", args, 1, tok) {
			return arrayOf(strT)
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "lines() expects str, got %s", args[0])
		}
		return arrayOf(strT)
	}}

	// ------------------------------------------------------------- numbers

	for _, name := range []string{"floor", "ceil", "round"} {
		n := name
		sigs[n] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
			if !c.expectArgs(n, args, 1, tok) {
				return intT
			}
			if args[0].Kind != TypeInt && args[0].Kind != TypeFloat && args[0].Kind != TypeAny {
				c.errorf(tok, "%s() expects int or float, got %s", n, args[0])
			}
			return intT
		}}
	}

	sigs["sqrt"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("sqrt", args, 1, tok) {
			return floatT
		}
		if args[0].Kind != TypeInt && args[0].Kind != TypeFloat && args[0].Kind != TypeAny {
			c.errorf(tok, "sqrt() expects int or float, got %s", args[0])
		}
		return floatT
	}}

	sigs["pow"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("pow", args, 2, tok) {
			return floatT
		}
		for i, a := range args {
			if a.Kind != TypeInt && a.Kind != TypeFloat && a.Kind != TypeAny {
				c.errorf(tok, "pow() argument %d expects int or float, got %s", i+1, a)
			}
		}
		// A whole base and a whole exponent stay whole at run time, so the
		// answer is only known to be a number here.
		if args[0].Kind == TypeInt && args[1].Kind == TypeInt {
			return intT
		}
		return floatT
	}}

	sigs["random"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		switch len(args) {
		case 0:
			return floatT
		case 1, 2:
			for i, a := range args {
				if a.Kind != TypeInt && a.Kind != TypeAny {
					c.errorf(tok, "random() argument %d expects int, got %s", i+1, a)
				}
			}
			return intT
		}
		c.errorf(tok, "random() takes 0, 1 or 2 argument(s), got %d", len(args))
		return floatT
	}}

	sigs["error"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("error", args, 1, tok) {
			return anyT
		}
		// error() accepts any value and inspects it as the error message.
		// error() never returns, so nothing after it is reached.
		return anyT
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
			// min(a) and max(a) over one array answer with an element.
			if len(args) == 1 && args[0].Kind == TypeArray {
				if args[0].Elem == nil {
					return anyT
				}
				return args[0].Elem
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

	sigs["input"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("input", args, 1, tok) {
			return strT
		}
		if args[0].Kind != TypeStr && args[0].Kind != TypeAny {
			c.errorf(tok, "input() expects str, got %s", args[0])
		}
		return strT
	}}

	sigs["read_all_stdin"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		c.expectArgs("read_all_stdin", args, 0, tok)
		return strT
	}}

	sigs["map"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("map", args, 2, tok) {
			return arrayOf(anyT)
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "map() expects array, got %s", args[0])
		}
		return arrayOf(anyT)
	}}

	sigs["filter"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("filter", args, 2, tok) {
			return arrayOf(anyT)
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "filter() expects array, got %s", args[0])
		}
		return args[0]
	}}

	sigs["reduce"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("reduce", args, 3, tok) {
			return anyT
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "reduce() expects array, got %s", args[0])
		}
		return args[2]
	}}

	sigs["any"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("any", args, 2, tok) {
			return boolT
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "any() expects array, got %s", args[0])
		}
		return boolT
	}}

	sigs["all"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("all", args, 2, tok) {
			return boolT
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "all() expects array, got %s", args[0])
		}
		return boolT
	}}

	sigs["sort_by"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("sort_by", args, 2, tok) {
			return arrayOf(anyT)
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "sort_by() expects array, got %s", args[0])
		}
		return args[0]
	}}

	sigs["find_index"] = &builtinSig{Check: func(c *Checker, args []*Type, tok Token) *Type {
		if !c.expectArgs("find_index", args, 2, tok) {
			return intT
		}
		if args[0].Kind != TypeArray && args[0].Kind != TypeAny {
			c.errorf(tok, "find_index() expects array, got %s", args[0])
		}
		return intT
	}}

	return sigs
}
