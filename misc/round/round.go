package round

import (
	"errors"
	"math"
	"sort"
)

var ErrRound = errors.New("can't round values to get sum")

const equalityEpsilon = 1e-9

type Value interface {
	GetFloatValue() float64
	SetFloatValue(value float64)
}

type Values []Value

type SortValues struct {
	values         []float64
	orginalIndexes []int
}

func newSortValues(values []Value) SortValues {
	indexes := make([]int, len(values))
	valuesCopy := make([]float64, len(values))

	for i := range values {
		indexes[i] = i
		valuesCopy[i] = values[i].GetFloatValue()
	}

	return SortValues{
		values:         valuesCopy,
		orginalIndexes: indexes,
	}
}

func (s SortValues) Len() int {
	return len(s.values)
}

func (s SortValues) Less(i, j int) bool {
	iDecimal, jDecimal := getDecimalPart(s.values[i]), getDecimalPart(s.values[j])

	if almostEqual(iDecimal, jDecimal) {
		return s.values[i] > s.values[j]
	}

	return iDecimal > jDecimal
}

func (s SortValues) Swap(i, j int) {
	s.values[i], s.values[j] = s.values[j], s.values[i]
	s.orginalIndexes[i], s.orginalIndexes[j] = s.orginalIndexes[j], s.orginalIndexes[i]
}

func getDecimalPart(x float64) float64 {
	return x - math.Trunc(x)
}

// SmartRound - method for rounding percents to integers
// in sum it must be 100% (can be changed in requiredSum argument)
// it's use larges reminder method but keep this extra requirement:
//   - if values is equal, after rounding the must be equal (if it possible)
//     example: 66.6666%, 16.666%, 16.666%
//     correct: 66, 17, 17
//     incorrect: 67, 17, 16
func SmartRound(values Values, requiredSum int) error {
	if len(values) == 0 || requiredSum < 0 {
		return ErrRound
	}
	for _, value := range values {
		if value == nil {
			return ErrRound
		}
		v := value.GetFloatValue()
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ErrRound
		}
	}

	sorted := newSortValues(values)

	// sort by decimal part, for equal decimal parts, use integer part
	sort.Sort(sorted)

	var actualSum float64

	// equalGroups it's a map for save info about equal values
	// equal values already grouped in array, because it's sorted
	equalGroups := make(map[int]int, 0)

	currentEqualGroupIndex := -1
	for i, value := range sorted.values {
		if i+1 < len(sorted.values) && almostEqual(sorted.values[i], sorted.values[i+1]) {
			if currentEqualGroupIndex >= 0 {
				equalGroups[currentEqualGroupIndex] = i + 1
			} else {
				equalGroups[i] = i + 1
				currentEqualGroupIndex = i
			}
		} else {
			currentEqualGroupIndex = -1
		}

		integerPart := math.Trunc(value)
		sorted.values[i] = integerPart
		actualSum += integerPart
	}

	diff := requiredSum - int(actualSum)
	if diff < 0 {
		return ErrRound
	}

	// save equality for groups if it possible
	for start, end := range equalGroups {
		if start < diff && end < diff {
			addOne(sorted.values, start, end)
			diff -= (end - start) + 1
		} else if start < diff && end >= diff {
			if diff-(end-start) > 0 {
				addOne(sorted.values, start, end)
				diff -= (end - start) + 1
			}
		}
	}

	if diff > len(values) {
		return ErrRound
	}

	if diff > 0 {
		for i := 0; i < len(values) && diff > 0; i++ {
			if end, exist := equalGroups[i]; exist {
				i = end
				continue
			}

			sorted.values[i]++
			diff--
		}
	}

	if diff > 0 {
		addOne(sorted.values, 0, diff-1)
	}

	if sumRounded(sorted.values) != requiredSum {
		return ErrRound
	}

	// restore original order for array
	for i := range sorted.values {
		originalIndex := sorted.orginalIndexes[i]

		originalValue := sorted.values[i]

		values[originalIndex].SetFloatValue(originalValue)
	}

	return nil
}

func addOne(arr []float64, start, end int) {
	for i := start; i <= end; i++ {
		arr[i]++
	}
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) <= equalityEpsilon
}

func sumRounded(values []float64) int {
	sum := 0
	for _, value := range values {
		sum += int(value)
	}

	return sum
}
