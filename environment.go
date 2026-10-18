package main

// Environment holds variable bindings for one scope, with a pointer to the
// enclosing scope so lookups can walk outwards.
type Environment struct {
	store map[string]Object
	outer *Environment

	// modules maps the aliases this scope can reach imported files by to those
	// files' own top-level scopes. A scope nested inside this one shares the
	// same map, so a function can use the import written above it.
	modules map[string]*Environment
}

// NewEnvironment creates a fresh top-level scope.
func NewEnvironment() *Environment {
	return &Environment{store: map[string]Object{}, modules: map[string]*Environment{}}
}

// NewEnclosedEnvironment creates a scope nested inside outer.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{store: map[string]Object{}, outer: outer, modules: outer.modules}
}

// Get looks a name up in this scope and then outwards.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return obj, ok
}

// Set creates or replaces a binding in this scope.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Assign updates an existing binding in the nearest scope that has it.
// It reports false when the name was never declared, which is how Jin
// catches typo'd assignments at runtime instead of silently creating globals.
func (e *Environment) Assign(name string, val Object) bool {
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return true
	}
	if e.outer != nil {
		return e.outer.Assign(name, val)
	}
	return false
}
