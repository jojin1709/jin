package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Instructions is compiled bytecode. Each opcode is one byte, followed by its
// operands, whose widths are listed in the table below.
type Instructions []byte

// Opcode is a single bytecode operation.
type Opcode byte

const (
	OpConstant Opcode = iota // push a constant from the pool
	OpTrue
	OpFalse
	OpNil
	OpPop          // discard the top of the stack
	OpPopStatement // discard a top-level statement's value, remembering it
	OpDup          // duplicate the top of the stack
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpEqual
	OpNotEqual
	OpGreater
	OpGreaterEqual
	OpLess
	OpLessEqual
	OpMinus
	OpBang
	OpInspect     // replace the top with its printed form
	OpLen         // replace the top with its length
	OpBuildString // join the top n values into one string
	OpJump        // move the instruction pointer
	OpJumpNotTruthy
	OpJumpIfTruthy
	OpGetGlobal
	OpSetGlobal
	OpGetLocal
	OpSetLocal
	OpGetFree
	OpSetFree
	OpGetBuiltin
	OpCurrentClosure // push the function currently running
	OpClosure        // build a closure, capturing free variables
	OpArray          // collect the top n values into an array
	OpBuildMap       // collect the top n key/value pairs into a map
	OpIterate        // replace the top with the values a for loop walks
	OpIndex          // container[index]
	OpSetIndex       // container[index] = value, leaving the value
	OpCall           // call a function with n arguments
	OpReturnValue    // return the top of the stack
)

type definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*definition{
	OpConstant:       {"OpConstant", []int{2}},
	OpTrue:           {"OpTrue", nil},
	OpFalse:          {"OpFalse", nil},
	OpNil:            {"OpNil", nil},
	OpPop:            {"OpPop", nil},
	OpPopStatement:   {"OpPopStatement", nil},
	OpDup:            {"OpDup", nil},
	OpAdd:            {"OpAdd", nil},
	OpSub:            {"OpSub", nil},
	OpMul:            {"OpMul", nil},
	OpDiv:            {"OpDiv", nil},
	OpMod:            {"OpMod", nil},
	OpEqual:          {"OpEqual", nil},
	OpNotEqual:       {"OpNotEqual", nil},
	OpGreater:        {"OpGreater", nil},
	OpGreaterEqual:   {"OpGreaterEqual", nil},
	OpLess:           {"OpLess", nil},
	OpLessEqual:      {"OpLessEqual", nil},
	OpMinus:          {"OpMinus", nil},
	OpBang:           {"OpBang", nil},
	OpInspect:        {"OpInspect", nil},
	OpLen:            {"OpLen", nil},
	OpBuildString:    {"OpBuildString", []int{2}},
	OpJump:           {"OpJump", []int{2}},
	OpJumpNotTruthy:  {"OpJumpNotTruthy", []int{2}},
	OpJumpIfTruthy:   {"OpJumpIfTruthy", []int{2}},
	OpGetGlobal:      {"OpGetGlobal", []int{2}},
	OpSetGlobal:      {"OpSetGlobal", []int{2}},
	OpGetLocal:       {"OpGetLocal", []int{1}},
	OpSetLocal:       {"OpSetLocal", []int{1}},
	OpGetFree:        {"OpGetFree", []int{1}},
	OpSetFree:        {"OpSetFree", []int{1}},
	OpGetBuiltin:     {"OpGetBuiltin", []int{1}},
	OpCurrentClosure: {"OpCurrentClosure", nil},
	OpClosure:        {"OpClosure", []int{2, 1}},
	OpArray:          {"OpArray", []int{2}},
	OpBuildMap:       {"OpBuildMap", []int{2}},
	OpIterate:        {"OpIterate", nil},
	OpIndex:          {"OpIndex", nil},
	OpSetIndex:       {"OpSetIndex", nil},
	OpCall:           {"OpCall", []int{1}},
	OpReturnValue:    {"OpReturnValue", nil},
}

// Lookup returns an opcode's definition.
func Lookup(op byte) (*definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

// Make encodes one instruction.
func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	length := 1
	for _, w := range def.OperandWidths {
		length += w
	}

	instruction := make([]byte, length)
	instruction[0] = byte(op)

	offset := 1
	for i, operand := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(operand))
		case 1:
			instruction[offset] = byte(operand)
		}
		offset += width
	}

	return instruction
}

// ReadUint16 reads a two-byte operand.
func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}

// ReadUint8 reads a one-byte operand.
func ReadUint8(ins Instructions) uint8 { return ins[0] }

// ReadOperands decodes the operands of one instruction, returning them and how
// many bytes they occupied.
func ReadOperands(def *definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		}
		offset += width
	}

	return operands, offset
}

// String renders bytecode as readable text, one instruction per line.
func (ins Instructions) String() string {
	var out strings.Builder
	i := 0

	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "%04d %s\n", i, err)
			i++
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		// A closure carries one (kind, index) pair per captured variable,
		// so those bytes are not instructions of their own.
		if Opcode(ins[i]) == OpClosure {
			count := operands[1]
			for f := 0; f < count; f++ {
				at := i + 1 + read + f*2
				if at+1 < len(ins) {
					operands = append(operands, int(ins[at]), int(ins[at+1]))
				}
			}
			read += count * 2
		}

		fmt.Fprintf(&out, "%04d %s\n", i, fmtInstruction(def, operands))
		i += 1 + read
	}

	return out.String()
}

func fmtInstruction(def *definition, operands []int) string {
	parts := make([]string, 0, len(operands)+1)
	parts = append(parts, def.Name)

	for _, o := range operands {
		parts = append(parts, fmt.Sprintf("%d", o))
	}

	return strings.Join(parts, " ")
}
