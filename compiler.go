package main

import (
	"fmt"
	"strings"
)

// Position is where a piece of code came from, so run-time errors can point at
// the right line and column even though the tree is gone.
type Position struct {
	Line int
	Col  int
}

// Bytecode is a compiled program ready for the VM.
type Bytecode struct {
	Instructions Instructions
	Positions    []Position
	Constants    []Object
	NumLocals    int
}

// constantPool is shared by every compiler in one program, so nested functions
// put their constants in the same table as the code that creates them.
type constantPool struct {
	items []Object
}

func (p *constantPool) add(obj Object) int {
	p.items = append(p.items, obj)
	return len(p.items) - 1
}

// loopContext is what break and continue need to know about the loop they sit
// in. Jumps are patched once the loop's layout is known.
type loopContext struct {
	continueJumps []int
	breaks        []int
}

// Compiler turns a syntax tree into bytecode. One compiler handles one
// function; nested functions get their own compiler that shares the constant
// pool and can reach out through enclosing for captured variables.
type Compiler struct {
	pool      *constantPool
	symbols   *SymbolTable
	enclosing *Compiler

	instructions Instructions
	positions    []Position
	position     Position // where the instruction being emitted came from

	functionName string
	loops        []*loopContext
}

// NewCompiler prepares a compiler that shares a global symbol table. Pass nil
// for globals to start a fresh one.
func NewCompiler(globals *SymbolTable) *Compiler {
	if globals == nil {
		globals = NewSymbolTable(true)
	}
	return &Compiler{
		pool:    &constantPool{},
		symbols: globals,
	}
}

// Compile turns a program into bytecode.
func (c *Compiler) Compile(program *Program) (*Bytecode, error) {
	// A REPL reuses one compiler, so each chunk starts from a clean slate.
	c.instructions = nil
	c.positions = nil
	c.loops = nil

	// Local slots do not survive between REPL lines, but globals do.
	c.symbols.ResetLocals()

	// Every top-level name is declared before anything is compiled, so a
	// function can use a global that is defined further down the file.
	for _, stmt := range program.Statements {
		switch node := stmt.(type) {
		case *LetStatement:
			c.symbols.Define(node.Name.Value)
		case *FunctionStatement:
			c.symbols.Define(node.Name.Value)
		}
	}

	for _, stmt := range program.Statements {
		if err := c.compileStatement(stmt); err != nil {
			return nil, err
		}
		// The script remembers the last statement's value so the REPL can
		// show it.
		c.emit(OpPopStatement)
	}

	c.emit(OpNil)
	c.emit(OpReturnValue)

	return &Bytecode{
		Instructions: c.instructions,
		Positions:    c.positions,
		Constants:    c.pool.items,
		NumLocals:    c.symbols.NumLocals(),
	}, nil
}

// ---------------------------------------------------------------- emitting

func (c *Compiler) addInstruction(ins []byte) int {
	pos := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	for range ins {
		c.positions = append(c.positions, c.position)
	}
	return pos
}

func (c *Compiler) emit(op Opcode, operands ...int) int {
	return c.addInstruction(Make(op, operands...))
}

// emitTwice adds two raw bytes, used by the closure instruction to list where
// each captured variable comes from.
func (c *Compiler) emitPair(a, b byte) {
	c.addInstruction([]byte{a, b})
}

func (c *Compiler) addConstant(obj Object) int {
	return c.pool.add(obj)
}

func (c *Compiler) changeOperand(pos int, operand int) {
	op := Opcode(c.instructions[pos])
	c.replaceInstruction(pos, Make(op, operand))
}

func (c *Compiler) replaceInstruction(pos int, ins []byte) {
	copy(c.instructions[pos:], ins)
}

// patchJump points a jump at the current end of the code.
func (c *Compiler) patchJump(pos int) {
	c.changeOperand(pos, len(c.instructions))
}

// ---------------------------------------------------------------- lookups

func (c *Compiler) emitGet(sym *Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpGetGlobal, sym.Index)
	case LocalScope:
		c.emit(OpGetLocal, sym.Index)
	case FreeScope:
		c.emit(OpGetFree, sym.Index)
	case BuiltinScope:
		c.emit(OpGetBuiltin, sym.Index)
	}
}

func (c *Compiler) emitSet(sym *Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpSetGlobal, sym.Index)
	case LocalScope:
		c.emit(OpSetLocal, sym.Index)
	case FreeScope:
		c.emit(OpSetFree, sym.Index)
	}
}

// resolve finds a name, creating a captured reference when the variable lives
// in an enclosing function.
func (c *Compiler) resolve(name string) (*Symbol, bool) {
	if sym, ok := c.symbols.Lookup(name); ok {
		return sym, true
	}

	if c.enclosing != nil {
		outer, ok := c.enclosing.resolve(name)
		if !ok {
			return nil, false
		}
		switch outer.Scope {
		case GlobalScope, BuiltinScope:
			return outer, true
		}
		index := c.symbols.DefineFreeNamed(name, freeRef{
			fromLocal: outer.Scope == LocalScope,
			index:     outer.Index,
		})
		return &Symbol{Name: name, Scope: FreeScope, Index: index}, true
	}

	if index, ok := builtinIndex(name); ok {
		return &Symbol{Name: name, Scope: BuiltinScope, Index: index}, true
	}
	return nil, false
}

// ---------------------------------------------------------------- statements

// statementToken finds the token a statement starts at, so every instruction
// it emits can point back to real source.
func statementToken(stmt Statement) Token {
	switch node := stmt.(type) {
	case *LetStatement:
		return node.Token
	case *FunctionStatement:
		return node.Token
	case *ReturnStatement:
		return node.Token
	case *BlockStatement:
		return node.Token
	case *ExpressionStatement:
		return node.Token
	case *IfStatement:
		return node.Token
	case *ForStatement:
		return node.Token
	case *WhileStatement:
		return node.Token
	case *BreakStatement:
		return node.Token
	case *ContinueStatement:
		return node.Token
	}
	return Token{}
}

func (c *Compiler) compileStatement(stmt Statement) error {
	if tok := statementToken(stmt); tok.Line > 0 {
		c.position = Position{tok.Line, tok.Column}
	}

	switch node := stmt.(type) {

	case *ExpressionStatement:
		return c.compileExpression(node.Expression)

	case *LetStatement:
		// A function written straight into a let takes the variable's name and
		// is declared first, so it can call itself.
		if lit, ok := node.Value.(*FunctionLiteral); ok {
			sym := c.symbols.Define(node.Name.Value)
			if err := c.compileFunction(lit.Parameters, lit.Body, node.Name.Value); err != nil {
				return err
			}
			c.emit(OpDup)
			c.emitSet(sym)
			return nil
		}

		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		sym := c.symbols.Define(node.Name.Value)
		c.emit(OpDup)
		c.emitSet(sym)
		return nil

	case *FunctionStatement:
		sym := c.symbols.Define(node.Name.Value)
		if err := c.compileFunction(node.Parameters, node.Body, node.Name.Value); err != nil {
			return err
		}
		c.emit(OpDup)
		c.emitSet(sym)
		return nil
	case *ReturnStatement:
		if node.Value == nil {
			c.emit(OpNil)
		} else if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		c.emit(OpReturnValue)
		return nil

	case *BlockStatement:
		c.symbols.EnterBlock()
		err := c.compileBlock(node, true)
		c.symbols.LeaveBlock()
		return err

	case *IfStatement:
		return c.compileIf(node)

	case *ForStatement:
		return c.compileFor(node)

	case *WhileStatement:
		return c.compileWhile(node)

	case *BreakStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("%d:%d: 'break' is only allowed inside a loop",
				node.Token.Line, node.Token.Column)
		}
		loop := c.loops[len(c.loops)-1]
		loop.breaks = append(loop.breaks, c.emit(OpJump, 0))
		return nil

	case *ContinueStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("%d:%d: 'continue' is only allowed inside a loop",
				node.Token.Line, node.Token.Column)
		}
		loop := c.loops[len(c.loops)-1]
		loop.continueJumps = append(loop.continueJumps, c.emit(OpJump, 0))
		return nil
	}

	return fmt.Errorf("cannot compile %T", stmt)
}

// compileBlock compiles statements in order. Every statement leaves one value;
// all but the last are discarded, and the last is kept when keepLast is set.
func (c *Compiler) compileBlock(block *BlockStatement, keepLast bool) error {
	if len(block.Statements) == 0 {
		c.emit(OpNil)
		return nil
	}

	for i, stmt := range block.Statements {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
		if i < len(block.Statements)-1 || !keepLast {
			c.emit(OpPop)
		}
	}
	return nil
}

func (c *Compiler) compileIf(node *IfStatement) error {
	c.position = Position{node.Token.Line, node.Token.Column}
	if err := c.compileExpression(node.Condition); err != nil {
		return err
	}

	jumpNotTruthy := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	c.symbols.EnterBlock()
	if err := c.compileBlock(node.Consequence, true); err != nil {
		return err
	}
	c.symbols.LeaveBlock()

	jumpToEnd := c.emit(OpJump, 0)

	c.patchJump(jumpNotTruthy)
	c.emit(OpPop)

	if node.Alternative != nil {
		if block, ok := node.Alternative.(*BlockStatement); ok {
			c.symbols.EnterBlock()
			err := c.compileBlock(block, true)
			c.symbols.LeaveBlock()
			if err != nil {
				return err
			}
		} else if err := c.compileStatement(node.Alternative); err != nil {
			return err
		}
	} else {
		c.emit(OpNil)
	}

	c.patchJump(jumpToEnd)
	return nil
}

func (c *Compiler) compileWhile(node *WhileStatement) error {
	loop := &loopContext{}
	c.loops = append(c.loops, loop)

	c.position = Position{node.Token.Line, node.Token.Column}
	start := len(c.instructions)

	if err := c.compileExpression(node.Condition); err != nil {
		return err
	}
	exitJump := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	c.symbols.EnterBlock()
	err := c.compileBlock(node.Body, false)
	c.symbols.LeaveBlock()
	if err != nil {
		return err
	}

	// continue jumps back to the condition.
	for _, pos := range loop.continueJumps {
		c.changeOperand(pos, start)
	}
	c.emit(OpJump, start)

	c.patchJump(exitJump)
	c.emit(OpPop)

	// break lands here, where the loop's value is waiting to be produced.
	breakTarget := len(c.instructions)
	for _, pos := range loop.breaks {
		c.changeOperand(pos, breakTarget)
	}
	c.emit(OpNil)

	c.loops = c.loops[:len(c.loops)-1]
	return nil
}

func (c *Compiler) compileFor(node *ForStatement) error {
	c.position = Position{node.Token.Line, node.Token.Column}

	if err := c.compileExpression(node.Iterable); err != nil {
		return err
	}
	// Turn whatever was given into something that can be walked by position,
	// so an array, a string and a map all use the same loop.
	c.emit(OpIterate)

	// The sequence and the position are kept in hidden slots.
	iterSym := c.symbols.DefineTemp()
	indexSym := c.symbols.DefineTemp()
	c.emitSet(iterSym)
	c.emit(OpConstant, c.addConstant(&Integer{Value: 0}))
	c.emitSet(indexSym)

	loop := &loopContext{}
	c.loops = append(c.loops, loop)

	start := len(c.instructions)

	c.emitGet(indexSym)
	c.emitGet(iterSym)
	c.emit(OpLen)
	c.emit(OpLess)
	exitJump := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	// Bind the loop variable for this pass. It is a fresh slot in a block
	// scope, so a closure made in the body captures this pass's value.
	c.symbols.EnterBlock()
	loopVar := c.symbols.Define(node.VarName.Value)
	c.emitGet(iterSym)
	c.emitGet(indexSym)
	c.emit(OpIndex)
	c.emitSet(loopVar)

	if err := c.compileBlock(node.Body, false); err != nil {
		return err
	}

	// continue jumps to the step below.
	stepTarget := len(c.instructions)
	for _, pos := range loop.continueJumps {
		c.changeOperand(pos, stepTarget)
	}

	c.emitGet(indexSym)
	c.emit(OpConstant, c.addConstant(&Integer{Value: 1}))
	c.emit(OpAdd)
	c.emitSet(indexSym)
	c.emit(OpJump, start)

	c.patchJump(exitJump)
	c.emit(OpPop)

	breakTarget := len(c.instructions)
	for _, pos := range loop.breaks {
		c.changeOperand(pos, breakTarget)
	}
	c.emit(OpNil)

	c.symbols.LeaveBlock()
	c.loops = c.loops[:len(c.loops)-1]
	return nil
}

// ---------------------------------------------------------------- expressions

func (c *Compiler) compileExpression(expr Expression) error {
	switch node := expr.(type) {

	case *IntegerLiteral:
		c.emit(OpConstant, c.addConstant(&Integer{Value: node.Value}))
		return nil

	case *FloatLiteral:
		c.emit(OpConstant, c.addConstant(&Float{Value: node.Value}))
		return nil

	case *BooleanLiteral:
		if node.Value {
			c.emit(OpTrue)
		} else {
			c.emit(OpFalse)
		}
		return nil

	case *NilLiteral:
		c.emit(OpNil)
		return nil

	case *StringLiteral:
		return c.compileString(node)

	case *Identifier:
		c.position = Position{node.Token.Line, node.Token.Column}

		// A function referring to itself uses the closure that is running,
		// which is the only way recursion works for a local function.
		if c.functionName != "" && node.Value == c.functionName {
			if sym, ok := c.resolve(node.Value); ok && sym.Scope != GlobalScope && sym.Scope != BuiltinScope {
				c.emit(OpCurrentClosure)
				return nil
			}
		}

		sym, ok := c.resolve(node.Value)
		if !ok {
			return fmt.Errorf("%d:%d: undefined variable %q", node.Token.Line, node.Token.Column, node.Value)
		}
		c.emitGet(sym)
		return nil

	case *PrefixExpression:
		c.position = Position{node.Token.Line, node.Token.Column}
		if err := c.compileExpression(node.Right); err != nil {
			return err
		}
		c.position = Position{node.Token.Line, node.Token.Column}
		switch node.Operator {
		case "!":
			c.emit(OpBang)
		case "-":
			c.emit(OpMinus)
		default:
			return fmt.Errorf("%d:%d: unknown operator %s", node.Token.Line, node.Token.Column, node.Operator)
		}
		return nil

	case *InfixExpression:
		c.position = Position{node.Token.Line, node.Token.Column}
		return c.compileInfix(node)

	case *AssignExpression:
		c.position = Position{node.Token.Line, node.Token.Column}
		return c.compileAssign(node)

	case *CallExpression:
		c.position = Position{node.Token.Line, node.Token.Column}
		if err := c.compileExpression(node.Function); err != nil {
			return err
		}
		for _, arg := range node.Arguments {
			if err := c.compileExpression(arg); err != nil {
				return err
			}
		}
		c.position = Position{node.Token.Line, node.Token.Column}
		c.emit(OpCall, len(node.Arguments))
		return nil

	case *IndexExpression:
		c.position = Position{node.Token.Line, node.Token.Column}
		if err := c.compileExpression(node.Left); err != nil {
			return err
		}
		if err := c.compileExpression(node.Index); err != nil {
			return err
		}
		c.position = Position{node.Token.Line, node.Token.Column}
		c.emit(OpIndex)
		return nil

	case *ArrayLiteral:
		for _, el := range node.Elements {
			if err := c.compileExpression(el); err != nil {
				return err
			}
		}
		c.emit(OpArray, len(node.Elements))
		return nil

	case *MapLiteral:
		// Keys and values are pushed in pairs, so the map can pop them back
		// in the order they were written.
		for _, pair := range node.Pairs {
			if err := c.compileExpression(pair.Key); err != nil {
				return err
			}
			if err := c.compileExpression(pair.Value); err != nil {
				return err
			}
		}
		c.emit(OpBuildMap, len(node.Pairs))
		return nil

	case *FunctionLiteral:
		return c.compileFunction(node.Parameters, node.Body, "")

	case *MemberExpression:
		return fmt.Errorf("%d:%d: property access (.%s) is not supported yet",
			node.Token.Line, node.Token.Column, node.Property.Value)
	}

	return fmt.Errorf("cannot compile %T", expr)
}

func (c *Compiler) compileInfix(node *InfixExpression) error {
	op := node.Operator

	// && and || only evaluate the right side when they have to.
	if op == "&&" || op == "||" {
		if err := c.compileExpression(node.Left); err != nil {
			return err
		}
		var jump int
		if op == "&&" {
			jump = c.emit(OpJumpNotTruthy, 0)
		} else {
			jump = c.emit(OpJumpIfTruthy, 0)
		}
		c.emit(OpPop)
		if err := c.compileExpression(node.Right); err != nil {
			return err
		}
		c.patchJump(jump)
		return nil
	}

	if err := c.compileExpression(node.Left); err != nil {
		return err
	}
	if err := c.compileExpression(node.Right); err != nil {
		return err
	}
	// Compiling the operands moved the position; point back at the operator.
	c.position = Position{node.Token.Line, node.Token.Column}

	switch op {
	case "+":
		c.emit(OpAdd)
	case "-":
		c.emit(OpSub)
	case "*":
		c.emit(OpMul)
	case "/":
		c.emit(OpDiv)
	case "%":
		c.emit(OpMod)
	case "==":
		c.emit(OpEqual)
	case "!=":
		c.emit(OpNotEqual)
	case ">":
		c.emit(OpGreater)
	case ">=":
		c.emit(OpGreaterEqual)
	case "<":
		c.emit(OpLess)
	case "<=":
		c.emit(OpLessEqual)
	default:
		return fmt.Errorf("%d:%d: unknown operator %s", node.Token.Line, node.Token.Column, op)
	}
	return nil
}

func (c *Compiler) compileAssign(node *AssignExpression) error {
	switch target := node.Target.(type) {

	case *Identifier:
		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		sym, ok := c.resolve(target.Value)
		if !ok {
			return fmt.Errorf("%d:%d: undefined variable %q; use 'let' to declare it first",
				node.Token.Line, node.Token.Column, target.Value)
		}
		if sym.Scope == BuiltinScope {
			return fmt.Errorf("%d:%d: cannot assign to built-in %q",
				node.Token.Line, node.Token.Column, target.Value)
		}
		c.emit(OpDup)
		c.emitSet(sym)
		return nil

	case *IndexExpression:
		if err := c.compileExpression(target.Left); err != nil {
			return err
		}
		if err := c.compileExpression(target.Index); err != nil {
			return err
		}
		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		c.position = Position{target.Token.Line, target.Token.Column}
		c.emit(OpSetIndex)
		return nil
	}

	return fmt.Errorf("%d:%d: cannot assign to this expression", node.Token.Line, node.Token.Column)
}

// compileString turns interpolation into bytecode: the text pieces become
// constants and the {expressions} are compiled like any other expression.
func (c *Compiler) compileString(node *StringLiteral) error {
	c.position = Position{node.Token.Line, node.Token.Column}

	if !strings.Contains(node.Value, "{") {
		c.emit(OpConstant, c.addConstant(&String{Value: node.Value}))
		return nil
	}

	segs, err := splitInterpolation(node.Value)
	if err != nil {
		return fmt.Errorf("%d:%d: %s", node.Token.Line, node.Token.Column, err)
	}

	count := 0
	for _, seg := range segs {
		if seg.Expr == "" {
			if seg.Literal == "" {
				continue
			}
			c.emit(OpConstant, c.addConstant(&String{Value: seg.Literal}))
			count++
			continue
		}

		p := NewParser(NewLexerAt(seg.Expr, node.Token.Line))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return fmt.Errorf("%d:%d: bad interpolation {%s}: %s",
				node.Token.Line, node.Token.Column, seg.Expr, strings.Join(p.Errors(), "; "))
		}
		if len(program.Statements) != 1 {
			return fmt.Errorf("%d:%d: interpolation {%s} must be one expression",
				node.Token.Line, node.Token.Column, seg.Expr)
		}
		stmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			return fmt.Errorf("%d:%d: interpolation {%s} must be an expression",
				node.Token.Line, node.Token.Column, seg.Expr)
		}
		if err := c.compileExpression(stmt.Expression); err != nil {
			return err
		}
		c.emit(OpInspect)
		count++
	}

	if count == 0 {
		c.emit(OpConstant, c.addConstant(&String{Value: ""}))
		return nil
	}
	c.emit(OpBuildString, count)
	return nil
}

// compileFunction compiles a function body into its own instruction stream and
// emits the code that builds the closure.
func (c *Compiler) compileFunction(params []*Parameter, body *BlockStatement, name string) error {
	child := &Compiler{
		pool:         c.pool,
		symbols:      NewSymbolTable(false),
		enclosing:    c,
		functionName: name,
	}
	child.position = Position{body.Token.Line, body.Token.Column}

	for _, p := range params {
		child.symbols.Define(p.Name.Value)
	}

	if err := child.compileBlock(body, true); err != nil {
		return err
	}
	child.emit(OpReturnValue)

	freeRefs := child.symbols.FreeRefs()
	fn := &CompiledFunction{
		Name:         name,
		Instructions: child.instructions,
		Positions:    child.positions,
		NumLocals:    child.symbols.NumLocals(),
		NumParams:    len(params),
	}

	c.emit(OpClosure, c.addConstant(fn), len(freeRefs))
	for _, ref := range freeRefs {
		kind := byte(0) // captured from the enclosing frame's locals
		if !ref.fromLocal {
			kind = 1 // captured from the enclosing closure's own captures
		}
		c.emitPair(kind, byte(ref.index))
	}
	return nil
}

// builtinIndexMap maps a built-in's name to the slot OpGetBuiltin uses.
var builtinIndexMap = func() map[string]int {
	m := make(map[string]int, len(builtinNames))
	for i, name := range builtinNames {
		m[name] = i
	}
	return m
}()

func builtinIndex(name string) (int, bool) {
	i, ok := builtinIndexMap[name]
	return i, ok
}
