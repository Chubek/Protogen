package protoparser

import (
	dtype "protogen/prototype"
	"regexp"
	"strings"
)

type precedence int

const (
	HighHighPrec precedence = 4
	LowHighPrec  precedence = 3
	HighLowPrec  precedence = 2
	LowLowPrec   precedence = 1
	ParanPrec    precedence = 0
	None         precedence = -1
)

var (
	precedences = map[string]precedence{
		"*": HighHighPrec,
		"/": LowHighPrec,
		"+": HighLowPrec,
		"-": LowLowPrec,
		"(": ParanPrec,
		")": ParanPrec,
		"":  None,
	}
)

func ShuntingYard(ex string) (string, bool) {
	tokenPatt := regexp.MustCompile(`\b\d+\b|\(|\)|\/|\-|\*|\+`)

	numberPatt := regexp.MustCompile(`\d+(\.\d+)?`)
	lParaPatt := regexp.MustCompile(`\(`)
	rParaPatt := regexp.MustCompile(`\)`)
	opPatt := regexp.MustCompile(`\/|\-|\*|\+`)

	tokens := tokenPatt.FindAllString(ex, -1)

	queueOut := dtype.NewQueue()
	stackOp := dtype.NewStack()

	for _, token := range tokens {
		token = strings.Trim(token, " ")
		if numberPatt.MatchString(token) {
			queueOut.Push(token, false)
		} else if opPatt.MatchString(token) {
			for precedences[stackOp.ReturnLast()] > precedences[token] {
				queueOut.Push(stackOp.Pop(), false)
			}

			stackOp.Push(token)
		} else if rParaPatt.MatchString(token) {

			for !lParaPatt.MatchString(stackOp.ReturnLast()) {
				tok_ := stackOp.Pop()
				queueOut.Push(tok_, false)
			}

			stackOp.Pop()

		} else {
			stackOp.Push(token)
		}
	}

	for !stackOp.IsEmpty() {
		queueOut.Push(stackOp.Pop(), false)
	}

	rpnStack := dtype.NewStack()

	var success bool

	for !queueOut.IsEmpty() {
		token := queueOut.Pop(false)

		if numberPatt.MatchString(token) {
			rpnStack.Push(token)
		} else {
			opOne := rpnStack.Pop()
			opTwo := rpnStack.Pop()

			success = rpnStack.DoOp(opOne, opTwo, token)
		}

	}

	return rpnStack.Pop(), success

}
