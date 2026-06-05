//nolint:recvcheck // Pgx encoders need value receivers and scanners need pointer receivers.
package pgtype

import (
	"encoding/json"
	"errors"
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
	errInvalidJSON = errors.New("invalid JSON")
	errUndefined   = errors.New("cannot encode status undefined")
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
