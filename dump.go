package main

import (
	"fmt"
	"strings"
)

// DumpProgram renders a syntax tree as indented text.
func DumpProgram(program *Program) string {
	var sb strings.Builder
	dumpNode(&sb, program, 0)
	return sb.String()
}

func dumpLine(sb *strings.Builder, indent int, format string, args ...any) {
	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString(fmt.Sprintf(format, args...))
	sb.WriteString("\n")
}

func dumpNode(sb *strings.Builder, n Node, indent int) {
	switch node := n.(type) {

	case *Program:
		dumpLine(sb, indent, "Program")
		for _, s := range node.Statements {
			dumpNode(sb, s, indent+1)
		}

	// ---- statements
	case *LetStatement:
		typePart := ""
		if node.Type != nil {
			typePart = " : " + node.Type.String()
		}
		dumpLine(sb, indent, "Let %s%s =", node.Name.Value, typePart)
		dumpNode(sb, node.Value, indent+1)

	case *FunctionStatement:
		dumpLine(sb, indent, "Function %s(%s)", node.Name.Value, joinParams(node.Parameters))
		if node.ReturnType != nil {
			dumpLine(sb, indent+1, "returns %s", node.ReturnType.String())
		}
		dumpNode(sb, node.Body, indent+1)

	case *ReturnStatement:
		dumpLine(sb, indent, "Return")
		if node.Value != nil {
			dumpNode(sb, node.Value, indent+1)
		}

	case *IfStatement:
		dumpLine(sb, indent, "If")
		dumpNode(sb, node.Condition, indent+1)
		dumpLine(sb, indent+1, "Then")
		dumpNode(sb, node.Consequence, indent+2)
		if node.Alternative != nil {
			dumpLine(sb, indent+1, "Else")
			dumpNode(sb, node.Alternative, indent+2)
		}

	case *ForStatement:
		dumpLine(sb, indent, "For %s in", node.VarName.Value)
		dumpNode(sb, node.Iterable, indent+1)
		dumpNode(sb, node.Body, indent+1)

	case *WhileStatement:
		dumpLine(sb, indent, "While")
		dumpNode(sb, node.Condition, indent+1)
		dumpNode(sb, node.Body, indent+1)

	case *BreakStatement:
		dumpLine(sb, indent, "Break")

	case *ContinueStatement:
		dumpLine(sb, indent, "Continue")

	case *BlockStatement:
		dumpLine(sb, indent, "Block")
		for _, s := range node.Statements {
			dumpNode(sb, s, indent+1)
		}

	case *ExpressionStatement:
		dumpLine(sb, indent, "Expr")
		dumpNode(sb, node.Expression, indent+1)

	// ---- expressions
	case *Identifier:
		dumpLine(sb, indent, "Ident %s", node.Value)

	case *IntegerLiteral:
		dumpLine(sb, indent, "Int %d", node.Value)

	case *FloatLiteral:
		dumpLine(sb, indent, "Float %g", node.Value)

	case *StringLiteral:
		dumpLine(sb, indent, "String %q", node.Value)

	case *BooleanLiteral:
		dumpLine(sb, indent, "Bool %t", node.Value)

	case *NilLiteral:
		dumpLine(sb, indent, "Nil")

	case *PrefixExpression:
		dumpLine(sb, indent, "Prefix %s", node.Operator)
		dumpNode(sb, node.Right, indent+1)

	case *InfixExpression:
		dumpLine(sb, indent, "Infix %s", node.Operator)
		dumpNode(sb, node.Left, indent+1)
		dumpNode(sb, node.Right, indent+1)

	case *AssignExpression:
		dumpLine(sb, indent, "Assign")
		dumpNode(sb, node.Target, indent+1)
		dumpNode(sb, node.Value, indent+1)

	case *CallExpression:
		dumpLine(sb, indent, "Call")
		dumpNode(sb, node.Function, indent+1)
		for _, a := range node.Arguments {
			dumpNode(sb, a, indent+1)
		}

	case *IndexExpression:
		dumpLine(sb, indent, "Index")
		dumpNode(sb, node.Left, indent+1)
		dumpNode(sb, node.Index, indent+1)

	case *MemberExpression:
		dumpLine(sb, indent, "Member .%s", node.Property.Value)
		dumpNode(sb, node.Object, indent+1)

	case *ArrayLiteral:
		dumpLine(sb, indent, "Array")
		for _, el := range node.Elements {
			dumpNode(sb, el, indent+1)
		}

	case *FunctionLiteral:
		dumpLine(sb, indent, "FunctionLiteral(%s)", joinParams(node.Parameters))
		if node.ReturnType != nil {
			dumpLine(sb, indent+1, "returns %s", node.ReturnType.String())
		}
		dumpNode(sb, node.Body, indent+1)

	default:
		dumpLine(sb, indent, "? %T", n)
	}
}
