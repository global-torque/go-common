package response

import (
	"encoding/json"
	"errors"
	"fmt"
)

const defaultErrorMessageKey = "__error__"

// ErrorMessages is the structured, JSON-safe error message shape used by Error.
type ErrorMessages struct {
	values map[string][]string
}

// swagger:model
type Error struct {
	StatusCode int           `json:"-"`
	Message    ErrorMessages `json:"message"`
	Err        error         `json:"-"`
}

func New(err error, status int, msg map[string][]string) Error {
	return Error{
		StatusCode: status,
		Err:        normalizeErr(err),
		Message:    NewErrorMessages(msg),
	}
}

func NewError(err error, args ...any) *Error {
	newError := Error{
		Err: normalizeErr(err),
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case int:
			newError.StatusCode = v
		case string:
			newError.Message = SingleErrorMessage(v)
		case ErrorMessages:
			newError.Message = v.Clone()
		case map[string][]string:
			newError.Message = NewErrorMessages(v)
		}
	}

	return &newError
}

func (r *Error) Error() string {
	if r == nil || r.Err == nil {
		return ""
	}

	return r.Err.Error()
}

func (r *Error) Unwrap() error {
	if r == nil {
		return nil
	}

	return r.Err
}

func (r *Error) GetMessageFromMap(key string) ([]string, bool) {
	if r == nil {
		return nil, false
	}

	return r.Message.Get(key)
}

func (r *Error) AddMessageToMap(key string, value string) {
	if r == nil {
		return
	}

	r.Message = r.Message.With(key, value)
}

func NewErrorMessages(values map[string][]string) ErrorMessages {
	return ErrorMessages{values: cloneMessageMap(values)}
}

func SingleErrorMessage(message string) ErrorMessages {
	return NewErrorMessages(map[string][]string{defaultErrorMessageKey: {message}})
}

func MessagesFromAny(message any) ErrorMessages {
	switch m := message.(type) {
	case nil:
		return ErrorMessages{}
	case ErrorMessages:
		return m.Clone()
	case map[string][]string:
		return NewErrorMessages(m)
	case string:
		return SingleErrorMessage(m)
	case error:
		return SingleErrorMessage(m.Error())
	case fmt.Stringer:
		return SingleErrorMessage(m.String())
	default:
		return SingleErrorMessage(fmt.Sprint(m))
	}
}

func (m ErrorMessages) Clone() ErrorMessages {
	return NewErrorMessages(m.values)
}

func (m ErrorMessages) Map() map[string][]string {
	return cloneMessageMap(m.values)
}

func (m ErrorMessages) Get(key string) ([]string, bool) {
	values, ok := m.values[key]
	if !ok {
		return nil, false
	}

	return append([]string(nil), values...), true
}

func (m ErrorMessages) With(key string, value string) ErrorMessages {
	next := cloneMessageMap(m.values)
	if next == nil {
		next = map[string][]string{}
	}
	next[key] = append(next[key], value)

	return NewErrorMessages(next)
}

func (m ErrorMessages) MarshalJSON() ([]byte, error) {
	if m.values == nil {
		return []byte("{}"), nil
	}

	return json.Marshal(m.values)
}

func normalizeErr(err error) error {
	if err != nil {
		return err
	}

	return errors.New("")
}

func cloneMessageMap(msg map[string][]string) map[string][]string {
	if msg == nil {
		return nil
	}

	cloned := make(map[string][]string, len(msg))
	for key, values := range msg {
		cloned[key] = append([]string(nil), values...)
	}

	return cloned
}
