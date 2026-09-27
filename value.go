package resp

import (
	"errors"
)

type Value struct {
	Typ         string
	StrValue    *string
	IntValue    int
	DoubleValue float64
	BoolValue   bool
	Verbatim    Verbatim
	ArrValues   *[]Value
	MapValues   Map
	SetValues   Set
}

type Verbatim struct {
	StrValue string
	Verbatim string
}

type Map interface {
	Get(key Value) (Value, error)
	Add(key Value, value Value) error
	Size() int
	Values() map[string]Value
}

type mapImpl struct {
	values map[string]Value
	w      Writer
}

func NewMap(size int) Map {
	return &mapImpl{
		values: make(map[string]Value, size),
	}
}

func (m *mapImpl) Size() int {
	return len(m.values)
}

func (m *mapImpl) Values() map[string]Value {
	return m.values
}

func (m *mapImpl) Get(key Value) (Value, error) {

	valStr, err := m.w.parseToString(key)
	if err != nil {
		return Value{}, err
	}

	return m.values[valStr], nil
}

func (m *mapImpl) Add(key Value, value Value) error {

	valStr, err := m.w.parseToString(key)
	if err != nil {
		return err
	}

	m.values[valStr] = value

	return nil
}

type Set interface {
	Get(key Value) (Value, error)
	Add(value Value) error
	Size() int
	Values() map[string]Value
}

type setImpl struct {
	values map[string]Value
	w      Writer
}

func NewSet(size int) Set {
	return &setImpl{
		values: make(map[string]Value, size),
	}
}

func (s *setImpl) Size() int {
	return len(s.values)
}

func (s *setImpl) Values() map[string]Value {
	return s.values
}

func (s *setImpl) Get(key Value) (Value, error) {

	valStr, err := s.w.parseToString(key)
	if err != nil {
		return Value{}, err
	}

	return s.values[valStr], nil
}

func (s *setImpl) Add(value Value) error {

	valStr, err := s.w.parseToString(value)
	if err != nil {
		return err
	}

	_, ok := s.values[valStr]
	if ok {
		return errors.New("such item already in a set")
	}
	s.values[valStr] = value
	return nil
}
