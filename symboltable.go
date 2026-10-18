package main

// SymbolScope says where a name's storage lives at run time.
type SymbolScope string

const (
	GlobalScope  SymbolScope = "GLOBAL"
	LocalScope   SymbolScope = "LOCAL"
	FreeScope    SymbolScope = "FREE"
	BuiltinScope SymbolScope = "BUILTIN"
)

// Symbol is a name the compiler has resolved.
type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

// freeRef records where a captured variable came from: either a local slot of
// the enclosing frame, or an entry in the enclosing closure's free list.
type freeRef struct {
	fromLocal bool
	index     int
}

// scopeFrame remembers the names a block introduced, so leaving the block can
// put back whatever it shadowed.
type scopeFrame struct {
	names     map[string]*Symbol
	numDefsAt int // local slots in use when this block opened
}

// SymbolTable holds the names visible to one function, plus the block scopes
// inside it. The root table holds globals and is shared by every compiler, so
// the REPL keeps its variables between lines.
type SymbolTable struct {
	store      map[string]*Symbol
	frames     []scopeFrame
	numDefs    int // local slots handed out in this function
	free       []freeRef
	freeByName map[string]int

	// Root tables also hand out global slots.
	isRoot     bool
	numGlobals int
}

// NewSymbolTable returns a table with one open block scope. Pass isRoot for
// the table that holds globals.
func NewSymbolTable(isRoot bool) *SymbolTable {
	return &SymbolTable{
		store:      map[string]*Symbol{},
		frames:     []scopeFrame{{names: map[string]*Symbol{}, numDefsAt: 0}},
		freeByName: map[string]int{},
		isRoot:     isRoot,
	}
}

// NumLocals reports how many slots this function's frame needs.
func (s *SymbolTable) NumLocals() int { return s.numDefs }

// ResetLocals forgets the local slots handed out so far. The top level uses
// this between REPL lines, where locals do not survive but globals do.
func (s *SymbolTable) ResetLocals() { s.numDefs = 0 }

// NumGlobals reports how many global slots have been handed out.
func (s *SymbolTable) NumGlobals() int { return s.numGlobals }

func (s *SymbolTable) depth() int { return len(s.frames) - 1 }

// EnterBlock opens a new, inner scope.
func (s *SymbolTable) EnterBlock() {
	s.frames = append(s.frames, scopeFrame{
		names:     map[string]*Symbol{},
		numDefsAt: s.numDefs,
	})
}

// LeaveBlock closes the innermost scope and restores any names it hid. Slots
// handed out inside the block are not reclaimed: the frame is sized by the
// highest slot ever used, so keeping the counter high is what makes it right.
func (s *SymbolTable) LeaveBlock() {
	frame := s.frames[len(s.frames)-1]
	for name, previous := range frame.names {
		if previous == nil {
			delete(s.store, name)
			continue
		}
		s.store[name] = previous
	}
	s.frames = s.frames[:len(s.frames)-1]
}

// Define declares a named variable. At the top level of a file that means a
// global; anywhere else it is a slot in the current frame. Redeclaring a name
// in the same block reuses its slot.
func (s *SymbolTable) Define(name string) *Symbol {
	top := &s.frames[len(s.frames)-1]
	if _, already := top.names[name]; already {
		return s.store[name]
	}
	top.names[name] = s.store[name] // remember what we are shadowing

	sym := &Symbol{Name: name}
	if s.isRoot && s.depth() == 0 {
		sym.Scope = GlobalScope
		sym.Index = s.numGlobals
		s.numGlobals++
	} else {
		sym.Scope = LocalScope
		sym.Index = s.numDefs
		s.numDefs++
	}

	s.store[name] = sym
	return sym
}

// DefineTemp hands out an unnamed local slot for the compiler's own use, such
// as a loop's counter.
func (s *SymbolTable) DefineTemp() *Symbol {
	sym := &Symbol{Name: "", Scope: LocalScope, Index: s.numDefs}
	s.numDefs++
	return sym
}

// DefineFree records that a variable from an enclosing function is captured,
// and returns its slot in this closure's free list.
func (s *SymbolTable) DefineFree(ref freeRef) int {
	s.free = append(s.free, ref)
	return len(s.free) - 1
}

// DefineFreeNamed is DefineFree, but a name captured more than once reuses the
// same slot.
func (s *SymbolTable) DefineFreeNamed(name string, ref freeRef) int {
	if index, ok := s.freeByName[name]; ok {
		return index
	}
	index := s.DefineFree(ref)
	s.freeByName[name] = index
	return index
}

// FreeRefs returns where each captured variable should be read from.
func (s *SymbolTable) FreeRefs() []freeRef { return s.free }

// Lookup finds a name declared in this table or an enclosing block of it.
func (s *SymbolTable) Lookup(name string) (*Symbol, bool) {
	sym, ok := s.store[name]
	return sym, ok
}
