package prototype

import (
	"strconv"
	"strings"
)

const NULLSTR = "\x00\x12\x01\x02"

type UsualType interface {
	int | string | byte | rune
}

type StrSlice []string

func minTuple(a, b int) int {
	if a > b {
		return b
	}

	return a
}

func rangeStep(start, stop, step int) ([]int, int) {
	ret := make([]int, 0)

	for i := start; i < stop; i += step {
		ret = append(ret, i)
	}

	return ret, len(ret)
}

func zipThroughIndices[UT UsualType](usl []UT) (map[int]int, bool) {
	length := len(usl)
	evensRange, lenEvens := rangeStep(0, length, 2)
	oddsRange, lenOdds := rangeStep(1, length, 2)

	ret := make(map[int]int, 0)

	for i := 0; i < minTuple(lenEvens, lenOdds); i++ {
		ret[evensRange[i]] = oddsRange[i]
	}

	return ret, lenEvens == lenOdds
}

func (ss StrSlice) ZipIt() map[string]string {
	ret := make(map[string]string, 0)
	indices, isUnEven := zipThroughIndices(ss)

	for bInd, aInd := range indices {
		ret[ss[bInd]] = ss[aInd]
	}

	if isUnEven {
		ret[ss[len(ss)-1]] = NULLSTR
	}

	return ret
}

type Stack []string

// IsEmpty: check if stack is empty
func (s *Stack) IsEmpty() bool {
	return len(*s) == 0
}

// IsEmpty: check if stack is empty
func (s *Stack) ReturnFirst() string {
	if len(*s) == 0 {
		return ""
	}

	return (*s)[0]
}

// IsEmpty: check if stack is empty
func (s *Stack) ReturnLast() string {
	if len(*s) == 0 {
		return ""
	}

	return (*s)[len(*s)-1]
}

// Push a new value onto the stack
func (s *Stack) Push(str string) {

	*s = append(*s, str) // Simply append the new value to the end of the stack
}

func (s *Stack) DoOp(opOne, opTwo, operator string) bool {
	var res float64

	firstParsed, successFirst := parseToFloat(opTwo)
	secondParsed, successSecond := parseToFloat(opOne)

	if !successFirst || !successSecond {
		return false
	}
	

	switch operator {
	case "+":
		res = firstParsed + secondParsed
	case "-":
		res = firstParsed - secondParsed
	case "*":
		res = firstParsed * secondParsed
	case "/":
		res = firstParsed / secondParsed

	}

	*s = append(*s, strconv.FormatFloat(res, 'f', 4, 64)) // Simply append the new value to the end of the stack

	return true
}

// Remove and return top element of stack. Return false if stack is empty.
func (s *Stack) Pop() string {
	if s.IsEmpty() {
		return ""
	} else {
		index := len(*s) - 1   // Get the index of the top most element.
		element := (*s)[index] // Index into the slice and obtain the element.
		*s = (*s)[:index]      // Remove it from the stack by slicing it off.

		return element
	}
}

func NewStack() *Stack {
	return &Stack{}
}

type Queue []string

func (q *Queue) IsEmpty() bool {
	return len(*q) == 0
}

func (q *Queue) HasAtLeastOne() bool {
	return len(*q) == 1
}

func (q *Queue) Push(xx string, op bool) {
	if !op {
		*q = append(*q, xx)
	} else {
		*q = append([]string{xx}, *q...)
	}

}

func (q *Queue) Pop(op bool) string {
	h := *q
	var el string
	l := len(h)

	if l == 1 {
		el, *q = h[0], []string{}
	} else if !op {
		el, *q = h[0], h[1:l]
	} else {
		el, *q = h[l-1], h[0:l-1]
	}

	return el
}

func NewQueue() *Queue {
	return &Queue{}
}

func parseToFloat(num string) (float64, bool) {
	numParsed, err := strconv.ParseFloat(strings.Trim(num, ""), 32)
	if err != nil {
		return 0.0, false
	}

	return numParsed, true
}
