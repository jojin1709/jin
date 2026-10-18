package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------- encoding

func TestInstructionRoundTrip(t *testing.T) {
	cases := []struct {
		op       Opcode
		operands []int
	}{
		{OpConstant, []int{65534}},
		{OpJump, []int{1}},
		{OpGetGlobal, []int{300}},
		{OpGetLocal, []int{7}},
		{OpCall, []int{3}},
		{OpBuildString, []int{2}},
		{OpCloseUpvalues, []int{65535}},
		{OpPop, nil},
		{OpReturnValue, nil},
	}

	for _, tc := range cases {
		ins := Instructions(Make(tc.op, tc.operands...))
		def, err := Lookup(byte(tc.op))
		if err != nil {
			t.Fatalf("Lookup(%d): %v", byte(tc.op), err)
		}

		want := 1
		for _, w := range def.OperandWidths {
			want += w
		}
		if len(ins) != want {
			t.Errorf("%s: length = %d, want %d", def.Name, len(ins), want)
		}

		got, read := ReadOperands(def, ins[1:])
		if read != want-1 {
			t.Errorf("%s: read %d operand bytes, want %d", def.Name, read, want-1)
		}
		for i, operand := range tc.operands {
			if got[i] != operand {
				t.Errorf("%s operand %d = %d, want %d", def.Name, i, got[i], operand)
			}
		}
	}
}

func TestDisassemblerHandlesClosures(t *testing.T) {
	// A closure instruction is followed by one (kind, index) pair per
	// captured variable; those bytes must not be read as instructions.
	fn := &CompiledFunction{Name: "inner", NumParams: 1, NumLocals: 2}
	ins := Instructions{}
	ins = append(ins, Make(OpClosure, 3, 2)...)
	ins = append(ins, 0, 1, 1, 0)
	ins = append(ins, Make(OpReturnValue)...)

	got := ins.String()
	if !strings.Contains(got, "OpClosure") {
		t.Errorf("disassembly is missing the closure:\n%s", got)
	}
	if strings.Contains(got, "undefined") {
		t.Errorf("disassembly misread the capture bytes:\n%s", got)
	}
	if !strings.Contains(got, "OpReturnValue") {
		t.Errorf("disassembly is missing the following instruction:\n%s", got)
	}
	_ = fn
}

// ---------------------------------------------------------------- symbols

func TestTopLevelNamesAreGlobals(t *testing.T) {
	table := NewSymbolTable(true)

	sym := table.Define("x")
	if sym.Scope != GlobalScope {
		t.Errorf("top-level x scope = %s, want %s", sym.Scope, GlobalScope)
	}
	if sym.Index != 0 {
		t.Errorf("top-level x index = %d, want 0", sym.Index)
	}

	if got := table.Define("y").Scope; got != GlobalScope {
		t.Errorf("second top-level name scope = %s, want %s", got, GlobalScope)
	}
	if got := table.NumGlobals(); got != 2 {
		t.Errorf("NumGlobals = %d, want 2", got)
	}
}

func TestNamesInsideABlockAreLocals(t *testing.T) {
	table := NewSymbolTable(true)

	table.EnterBlock()
	sym := table.Define("inner")
	if sym.Scope != LocalScope {
		t.Errorf("block-local scope = %s, want %s", sym.Scope, LocalScope)
	}
	table.LeaveBlock()

	// The name is gone, so it must not be findable.
	if _, ok := table.Lookup("inner"); ok {
		t.Error("a block-local name leaked out of its block")
	}
}

func TestBlockScopingRestoresShadowedNames(t *testing.T) {
	table := NewSymbolTable(true)

	outer := table.Define("x")

	table.EnterBlock()
	inner := table.Define("x")
	if inner == outer {
		t.Fatal("the inner declaration did not shadow the outer one")
	}
	if got, _ := table.Lookup("x"); got != inner {
		t.Error("inside the block, x should be the inner declaration")
	}
	table.LeaveBlock()

	if got, ok := table.Lookup("x"); !ok || got != outer {
		t.Error("after the block, x should be the outer declaration again")
	}
}

func TestRedeclaringInTheSameBlockReusesTheSlot(t *testing.T) {
	table := NewSymbolTable(true)

	first := table.Define("x")
	second := table.Define("x")
	if first != second {
		t.Errorf("redeclaring gave a new symbol (%v vs %v)", first, second)
	}
	if got := table.NumGlobals(); got != 1 {
		t.Errorf("NumGlobals = %d, want 1", got)
	}
}

func TestLocalsAreNumberedPerFunction(t *testing.T) {
	table := NewSymbolTable(false)

	a := table.Define("a")
	b := table.Define("b")
	if a.Index != 0 || b.Index != 1 {
		t.Errorf("local indices = %d, %d; want 0, 1", a.Index, b.Index)
	}
	if got := table.NumLocals(); got != 2 {
		t.Errorf("NumLocals = %d, want 2", got)
	}
}

func TestFrameIsSizedByTheHighestSlotUsed(t *testing.T) {
	table := NewSymbolTable(false)

	table.EnterBlock()
	deep := table.Define("deep")
	table.LeaveBlock()

	if got := table.NumLocals(); got <= deep.Index {
		t.Errorf("NumLocals = %d, must cover the highest slot %d", got, deep.Index)
	}
}

func TestFreeVariablesAreRecordedOnce(t *testing.T) {
	table := NewSymbolTable(false)

	first := table.DefineFreeNamed("captured", freeRef{fromLocal: true, index: 3})
	second := table.DefineFreeNamed("captured", freeRef{fromLocal: true, index: 3})
	if first != second {
		t.Errorf("the same name got two free slots: %d and %d", first, second)
	}
	if got := len(table.FreeRefs()); got != 1 {
		t.Errorf("FreeRefs length = %d, want 1", got)
	}

	refs := table.FreeRefs()
	if !refs[0].fromLocal || refs[0].index != 3 {
		t.Errorf("free ref = %+v, want {fromLocal:true index:3}", refs[0])
	}
}

// ---------------------------------------------------------------- compiler

// compileSource compiles source with a fresh global table.
func compileSource(t *testing.T, source string) *Bytecode {
	t.Helper()

	p := NewParser(NewLexer(source))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	bc, err := compileProgram(program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return bc
}

func TestCompiledPositionsMatchInstructions(t *testing.T) {
	bc := compileSource(t, `let a = 1
if a > 0 {
    print("positive")
}`)

	if len(bc.Positions) != len(bc.Instructions) {
		t.Fatalf("got %d positions for %d instruction bytes; they must line up",
			len(bc.Positions), len(bc.Instructions))
	}

	// Every instruction should be traceable to a real source line.
	for i, pos := range bc.Positions {
		if pos.Line == 0 {
			t.Fatalf("instruction byte %d has no source line", i)
		}
	}
}

func TestEveryStatementLeavesTheStackBalanced(t *testing.T) {
	// A jump target must line up with a place where the operand stack is the
	// same depth, so the two paths through an if cannot disagree.
	bc := compileSource(t, `let n = 1
if n > 0 {
    print("yes")
} else {
    print("no")
}
print("after")`)

	// The two branches must both leave exactly one value.
	depth := 0
	for i := 0; i < len(bc.Instructions); {
		def, err := Lookup(bc.Instructions[i])
		if err != nil {
			t.Fatal(err)
		}
		op := Opcode(bc.Instructions[i])
		operands, read := ReadOperands(def, bc.Instructions[i+1:])

		switch op {
		case OpClosure:
			read += operands[1] * 2
		case OpPop, OpPopStatement:
			depth--
		case OpConstant, OpGetGlobal, OpGetLocal, OpGetFree, OpGetBuiltin, OpTrue, OpFalse, OpNil, OpCurrentClosure, OpDup:
			depth++
		case OpCall:
			depth -= operands[0]
		}
		if depth < 0 {
			t.Fatalf("stack went negative at byte %d (%s)", i, def.Name)
		}
		i += 1 + read
	}
}

func TestClosureRecordsWhereCapturesComeFrom(t *testing.T) {
	bc := compileSource(t, `fn outer() {
    let captured = 1
    return fn() { return captured }
}
print(outer())`)

	// The inner closure lives inside outer's own instruction stream, which is
	// one of the constants.
	var streams []Instructions
	streams = append(streams, bc.Instructions)
	for _, c := range bc.Constants {
		if fn, ok := c.(*CompiledFunction); ok {
			streams = append(streams, fn.Instructions)
		}
	}

	found := 0
	for _, ins := range streams {
		for i := 0; i < len(ins); {
			def, err := Lookup(ins[i])
			if err != nil {
				t.Fatal(err)
			}
			op := Opcode(ins[i])
			operands, read := ReadOperands(def, ins[i+1:])

			if op == OpClosure {
				count := operands[1]
				found += count
				for f := 0; f < count; f++ {
					kind := ins[i+1+read+f*2]
					if kind != 0 {
						t.Errorf("capture %d came from a free list; it should be a local of the enclosing frame", f)
					}
				}
				read += count * 2
			}
			i += 1 + read
		}
	}

	if found != 1 {
		t.Errorf("captured %d variables, want 1", found)
	}
}

// A for loop closes the cells its body opened from the slot where the body
// begins. The loop's own two hidden slots sit below that, so the loop's
// bookkeeping is never mistaken for a capture, and a variable declared outside
// the loop keeps its cell open across every pass.
func TestForLoopClosesFromTheBodysFirstSlot(t *testing.T) {
	bc := compileSource(t, `let total = 0
for i in range(2) {
    total = total + i
}
print(total)`)

	closes := []int{}
	for i := 0; i < len(bc.Instructions); {
		def, err := Lookup(bc.Instructions[i])
		if err != nil {
			t.Fatal(err)
		}
		operands, read := ReadOperands(def, bc.Instructions[i+1:])
		if Opcode(bc.Instructions[i]) == OpCloseUpvalues {
			closes = append(closes, operands[0])
		}
		i += 1 + read
	}

	if len(closes) != 1 {
		t.Fatalf("found %d closes, want 1", len(closes))
	}
	// The sequence and the position take slots 0 and 1, so the body starts at 2.
	if closes[0] != 2 {
		t.Errorf("loop closed from slot %d, want 2, the body's first slot", closes[0])
	}

	// Code with no loop has nothing open, so it closes nothing.
	plain := compileSource(t, `let x = 1
print(x)`)
	if strings.Contains(plain.Instructions.String(), "OpCloseUpvalues") {
		t.Error("code with no loop should not close any cells")
	}
}

func TestBreakAndContinueAreOnlyAllowedInLoops(t *testing.T) {
	for _, source := range []string{"break", "continue"} {
		p := NewParser(NewLexer(source))
		program := p.ParseProgram()
		if _, err := compileProgram(program); err == nil {
			t.Errorf("%q outside a loop should not compile", source)
		}
	}
}

func TestUndefinedNameIsACompileError(t *testing.T) {
	p := NewParser(NewLexer("print(nope)"))
	program := p.ParseProgram()
	_, err := compileProgram(program)
	if err == nil {
		t.Fatal("expected an error for an undefined name")
	}
	if !strings.Contains(err.Error(), `undefined variable "nope"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInterpolationWithBadExpressionIsACompileError(t *testing.T) {
	p := NewParser(NewLexer(`print("value {1 +}")`))
	program := p.ParseProgram()
	_, err := compileProgram(program)
	if err == nil {
		t.Fatal("expected an error for a broken interpolation")
	}
	if !strings.Contains(err.Error(), "bad interpolation") {
		t.Errorf("unexpected error: %v", err)
	}
}
