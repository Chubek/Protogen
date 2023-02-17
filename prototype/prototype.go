package prototype

const NULLSTR = "\x00\x12\x01\x02"

type UsualType interface {
	int | string | byte
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
