package evaluator

import (
	"fmt"
	"github.com/st0012/rooby/ast"
	"github.com/st0012/rooby/object"
)

var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
)

func Eval(node ast.Node, scope *object.Scope) object.Object {
	switch node := node.(type) {

	// Statements
	case *ast.Program:
		return evalProgram(node.Statements, scope)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, scope)
	case *ast.BlockStatement:
		return evalBlockStatements(node.Statements, scope)
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, scope)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		return evalLetStatement(node, scope)
	case *ast.ClassStatement:
		return evalClassStatement(node, scope)
	case *ast.Identifier:
		return evalIdentifier(node, scope)
	case *ast.Constant:
		return evalConstant(node, scope)
	case *ast.InstanceVariable:
		return evalInstanceVariable(node, scope)
	case *ast.DefStatement:
		return evalDefStatement(node, scope)

	// Expressions
	case *ast.IfExpression:
		return evalIfExpression(node, scope)
	case *ast.CallExpression:
		receiver := Eval(node.Receiver, scope)
		args := evalArgs(node.Arguments, scope)
		return sendMethodCall(receiver, node.Method.Value, args)

	case *ast.PrefixExpression:
		val := Eval(node.Right, scope)
		if isError(val) {
			return val
		}
		return evalPrefixExpression(node.Operator, val)
	case *ast.InfixExpression:
		valLeft := Eval(node.Left, scope)
		if isError(valLeft) {
			return valLeft
		}

		valRight := Eval(node.Right, scope)
		if isError(valRight) {
			return valRight
		}

		return evalInfixExpression(valLeft, node.Operator, valRight)
	case *ast.SelfExpression:
		return scope.Self
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.Boolean:
		if node.Value {
			return TRUE
		}
		return FALSE
	}

	return nil
}

func evalProgram(stmts []ast.Statement, scope *object.Scope) object.Object {
	var result object.Object

	for _, statement := range stmts {
		result = Eval(statement, scope)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func sendMethodCall(receiver object.Object, method_name string, args []object.Object) object.Object {
	switch receiver := receiver.(type) {
	case *object.Class:
		evaluated := evalClassMethod(receiver, method_name, args)

		return unwrapReturnValue(evaluated)
	case *object.BaseObject:
		evaluated := evalInstanceMethod(receiver, method_name, args)

		return unwrapReturnValue(evaluated)
	default:
		return newError("not a valid receiver: %s", receiver.Inspect())
	}
}

func evalClassMethod(receiver *object.Class, method_name string, args []object.Object) object.Object {
	method, ok := receiver.ClassMethods.Get(method_name)

	if !ok {
		if receiver.SuperClass == nil {
			return &object.Error{Message: fmt.Sprintf("undefined method %s for class %s", method_name, receiver.Inspect())}
		} else {
			method = evalClassMethod(receiver.SuperClass, method_name, args)
		}
	}

	switch m := method.(type) {
	case *object.Method:
		if len(m.Parameters) != len(args) {
			return newError("wrong arguments: expect=%d, got=%d", len(m.Parameters), len(args))
		}

		methodEnv := extendMethodEnv(m, args)
		scope := &object.Scope{Self: receiver, Env: methodEnv}
		return Eval(m.Body, scope)
	case *object.BuiltInMethod:
		return m.Fn(args...)
	default:
		return newError("unknown method type")
	}

}

func evalInstanceMethod(receiver *object.BaseObject, method_name string, args []object.Object) object.Object {
	class := receiver.Class
	method, ok := class.InstanceMethods.Get(method_name)

	if !ok {
		for class != nil {
			method, ok = class.InstanceMethods.Get(method_name)

			if !ok {
				// search superclass's superclass
				class = class.SuperClass

				// but if no more superclasses, return an error.
				if class == nil {
					return &object.Error{Message: fmt.Sprintf("undefined instance method %s for class %s", method_name, receiver.Class.Inspect())}
				}
			} else {
				// stop looping
				class = nil
			}
		}
	}

	switch m := method.(type) {
	case *object.Method:
		if len(m.Parameters) != len(args) {
			return newError("wrong arguments: expect=%d, got=%d", len(m.Parameters), len(args))
		}

		methodEnv := extendMethodEnv(m, args)
		scope := &object.Scope{Self: receiver, Env: methodEnv}
		return Eval(m.Body, scope)
	default:
		return newError("unknown method type")
	}

}

func evalArgs(exps []ast.Expression, scope *object.Scope) []object.Object {
	args := []object.Object{}

	for _, exp := range exps {
		arg := Eval(exp, scope)
		args = append(args, arg)
		if isError(arg) {
			return []object.Object{arg}
		}
	}

	return args
}

func extendMethodEnv(method *object.Method, args []object.Object) *object.Environment {
	e := object.NewClosedEnvironment(method.Scope.Env)

	for i, arg := range args {
		argName := method.Parameters[i].Value
		e.Set(argName, arg)
	}

	return e
}

func newError(format string, args ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}