# misc/round

Import path: `github.com/global-torque/go-common/misc/v2/round`

Rounds float values to integer-like values while preserving a required total,
using a largest-remainder strategy with special handling for equal values.

## Use For

- Percentages that must sum to exactly `100`.
- Weights or allocations that must round to a fixed integer sum.

## Do Not Use For

- Currency calculations.
- Arbitrary precision decimal math.
- Inputs containing negative totals, `NaN`, or infinity.

## Key APIs

- `Value`
- `Values`
- `SmartRound(values, requiredSum)`
- `ErrRound`

## Wiring Pattern

```go
type Part struct {
	Percent float64
}

func (p *Part) GetFloatValue() float64      { return p.Percent }
func (p *Part) SetFloatValue(value float64) { p.Percent = value }

values := round.Values{&Part{33.3}, &Part{33.3}, &Part{33.4}}
err := round.SmartRound(values, 100)
```

## Testing

The package tests cover equal-value groups, impossible input, and real
percentage cases.

## Gotchas

- `SmartRound` mutates the passed values through `SetFloatValue`.
- Empty input, nil values, non-finite numbers, negative required sums, and
  impossible totals return `ErrRound`.
