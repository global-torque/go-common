//nolint:recvcheck // Pgx encoders need value receivers and scanners need pointer receivers.
package pgtype

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	pgxpgtype "github.com/jackc/pgx/v5/pgtype"
)

// Status represents whether a pgtype value is undefined, null, or present.
type Status byte

const (
	// Undefined is the zero status for values that have not been set.
	Undefined Status = iota
	// Null represents a database NULL.
	Null
	// Present represents a non-null database value.
	Present
)

// InfinityModifier describes finite and infinite timestamp values.
type InfinityModifier int8

const (
	// Infinity represents positive infinity.
	Infinity InfinityModifier = 1
	// None represents a finite timestamp.
	None InfinityModifier = 0
	// NegativeInfinity represents negative infinity.
	NegativeInfinity InfinityModifier = -Infinity
)

var (
	errInvalidJSON            = errors.New("invalid JSON")
	errUndefined              = errors.New("cannot encode status undefined")
	errCannotConvertTimestamp = errors.New("cannot convert to Timestamptz")
	errUseJSONPointer         = errors.New("use pointer to pgtype.JSON instead of value")
	errUseJSONBPointer        = errors.New("use pointer to pgtype.JSONB instead of value")
	errCannotAssignNonPresent = errors.New("cannot assign non-present status")
)

// Text represents PostgreSQL text with old pgtype Status semantics.
type Text struct {
	String string
	Status Status
}

// ScanText implements pgx text scanning.
func (t *Text) ScanText(src pgxpgtype.Text) error {
	if !src.Valid {
		*t = Text{Status: Null}
		return nil
	}

	*t = Text{String: src.String, Status: Present}

	return nil
}

// TextValue implements pgx text encoding.
func (t Text) TextValue() (pgxpgtype.Text, error) {
	switch t.Status {
	case Present:
		return pgxpgtype.Text{String: t.String, Valid: true}, nil
	case Null:
		return pgxpgtype.Text{}, nil
	case Undefined:
		return pgxpgtype.Text{}, errUndefined
	default:
		return pgxpgtype.Text{}, errUndefined
	}
}

// Timestamptz represents PostgreSQL timestamptz with old pgtype Status semantics.
type Timestamptz struct {
	Time             time.Time
	Status           Status
	InfinityModifier InfinityModifier
}

// Set converts and assigns src using the legacy pgtype Value method shape.
func (t *Timestamptz) Set(src any) error {
	if src == nil {
		*t = Timestamptz{Status: Null}
		return nil
	}

	switch value := src.(type) {
	case time.Time:
		*t = Timestamptz{Time: value, Status: Present}
	case *time.Time:
		if value == nil {
			*t = Timestamptz{Status: Null}
			return nil
		}

		return t.Set(*value)
	case InfinityModifier:
		*t = Timestamptz{InfinityModifier: value, Status: Present}
	default:
		if converted, ok := underlyingTime(src); ok {
			return t.Set(converted)
		}

		return fmt.Errorf("%w: %v", errCannotConvertTimestamp, value)
	}

	return nil
}

// ScanTimestamptz implements pgx timestamptz scanning.
func (t *Timestamptz) ScanTimestamptz(src pgxpgtype.Timestamptz) error {
	if !src.Valid {
		*t = Timestamptz{Status: Null}
		return nil
	}

	*t = Timestamptz{
		Time:             src.Time,
		Status:           Present,
		InfinityModifier: fromPgxInfinity(src.InfinityModifier),
	}

	return nil
}

// TimestamptzValue implements pgx timestamptz encoding.
func (t Timestamptz) TimestamptzValue() (pgxpgtype.Timestamptz, error) {
	switch t.Status {
	case Present:
		return pgxpgtype.Timestamptz{
			Time:             t.Time,
			InfinityModifier: toPgxInfinity(t.InfinityModifier),
			Valid:            true,
		}, nil
	case Null:
		return pgxpgtype.Timestamptz{}, nil
	case Undefined:
		return pgxpgtype.Timestamptz{}, errUndefined
	default:
		return pgxpgtype.Timestamptz{}, errUndefined
	}
}

// JSON represents PostgreSQL json bytes with old pgtype Status semantics.
type JSON struct {
	Bytes  []byte
	Status Status
}

// ScanBytes implements pgx JSON scanning.
func (j *JSON) ScanBytes(src []byte) error {
	if src == nil {
		*j = JSON{Status: Null}
		return nil
	}

	j.Bytes = append(j.Bytes[:0], src...)
	j.Status = Present

	return nil
}

// BytesValue returns JSON bytes for pgx bytea-compatible codecs.
func (j JSON) BytesValue() ([]byte, error) {
	switch j.Status {
	case Present:
		return j.Bytes, nil
	case Null:
		return nil, nil
	case Undefined:
		return nil, errUndefined
	default:
		return nil, errUndefined
	}
}

// MarshalJSON returns the raw JSON payload for pgx JSON codecs.
func (j JSON) MarshalJSON() ([]byte, error) {
	switch j.Status {
	case Present:
		if !json.Valid(j.Bytes) {
			return nil, errInvalidJSON
		}

		return j.Bytes, nil
	case Null:
		return []byte("null"), nil
	case Undefined:
		return nil, errUndefined
	default:
		return nil, errUndefined
	}
}

// JSONB represents PostgreSQL jsonb bytes with old pgtype Status semantics.
type JSONB JSON

// Set converts and assigns src using the legacy pgtype Value method shape.
func (j *JSONB) Set(src any) error {
	if src == nil {
		*j = JSONB{Status: Null}
		return nil
	}

	switch value := src.(type) {
	case string:
		*j = JSONB{Bytes: []byte(value), Status: Present}
	case *string:
		if value == nil {
			*j = JSONB{Status: Null}
			return nil
		}

		*j = JSONB{Bytes: []byte(*value), Status: Present}
	case []byte:
		if value == nil {
			*j = JSONB{Status: Null}
			return nil
		}

		*j = JSONB{Bytes: value, Status: Present}
	case JSON:
		return errUseJSONPointer
	case JSONB:
		return errUseJSONBPointer
	default:
		buf, err := json.Marshal(value)
		if err != nil {
			return err
		}

		*j = JSONB{Bytes: buf, Status: Present}
	}

	return nil
}

// AssignTo converts and assigns the JSONB value to dst.
func (j *JSONB) AssignTo(dst any) error {
	switch value := dst.(type) {
	case *string:
		if j.Status != Present {
			return fmt.Errorf("%w to %T", errCannotAssignNonPresent, dst)
		}

		*value = string(j.Bytes)
	case **string:
		if j.Status == Present {
			str := string(j.Bytes)
			*value = &str

			return nil
		}

		*value = nil
	case *[]byte:
		if j.Status != Present {
			*value = nil
			return nil
		}

		buf := make([]byte, len(j.Bytes))
		copy(buf, j.Bytes)
		*value = buf
	default:
		data := j.Bytes
		if data == nil || j.Status != Present {
			data = []byte("null")
		}

		target := reflect.ValueOf(dst).Elem()
		target.Set(reflect.Zero(target.Type()))

		return json.Unmarshal(data, dst)
	}

	return nil
}

// ScanBytes implements pgx JSONB scanning.
func (j *JSONB) ScanBytes(src []byte) error {
	if src == nil {
		*j = JSONB{Status: Null}
		return nil
	}

	j.Bytes = append(j.Bytes[:0], src...)
	j.Status = Present

	return nil
}

// BytesValue returns JSONB bytes for pgx bytea-compatible codecs.
func (j JSONB) BytesValue() ([]byte, error) {
	switch j.Status {
	case Present:
		return j.Bytes, nil
	case Null:
		return nil, nil
	case Undefined:
		return nil, errUndefined
	default:
		return nil, errUndefined
	}
}

// MarshalJSON returns the raw JSONB payload for pgx JSON codecs.
func (j JSONB) MarshalJSON() ([]byte, error) {
	return JSON(j).MarshalJSON()
}

func fromPgxInfinity(value pgxpgtype.InfinityModifier) InfinityModifier {
	switch value {
	case pgxpgtype.Infinity:
		return Infinity
	case pgxpgtype.NegativeInfinity:
		return NegativeInfinity
	case pgxpgtype.Finite:
		return None
	default:
		return None
	}
}

func toPgxInfinity(value InfinityModifier) pgxpgtype.InfinityModifier {
	switch value {
	case Infinity:
		return pgxpgtype.Infinity
	case NegativeInfinity:
		return pgxpgtype.NegativeInfinity
	case None:
		return pgxpgtype.Finite
	default:
		return pgxpgtype.Finite
	}
}

func underlyingTime(value any) (time.Time, bool) {
	refValue := reflect.ValueOf(value)
	if !refValue.IsValid() {
		return time.Time{}, false
	}

	if refValue.Kind() == reflect.Pointer {
		if refValue.IsNil() {
			return time.Time{}, false
		}

		return underlyingTime(refValue.Elem().Interface())
	}

	timeType := reflect.TypeFor[time.Time]()
	if refValue.Type().ConvertibleTo(timeType) {
		converted, ok := refValue.Convert(timeType).Interface().(time.Time)

		return converted, ok
	}

	return time.Time{}, false
}
