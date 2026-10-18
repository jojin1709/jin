package main

// Frame is one active function call: the closure being run, how far into its
// instructions we are, and where its local variables start on the value stack.
type Frame struct {
	cl          *Closure
	ip          int
	basePointer int
}
