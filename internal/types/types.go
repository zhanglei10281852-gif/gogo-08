package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ValueType int

const (
	TypeNull ValueType = iota
	TypeText
	TypeInt
	TypeFloat
	TypeBool
	TypeDate
	TypeDateTime
)

func (t ValueType) String() string {
	switch t {
	case TypeNull:
		return "NULL"
	case TypeText:
		return "TEXT"
	case TypeInt:
		return "INT"
	case TypeFloat:
		return "FLOAT"
	case TypeBool:
		return "BOOL"
	case TypeDate:
		return "DATE"
	case TypeDateTime:
		return "DATETIME"
	default:
		return "UNKNOWN"
	}
}

type Value struct {
	Type ValueType
	V    interface{}
}

func NullValue() Value { return Value{Type: TypeNull} }
func TextValue(s string) Value { return Value{Type: TypeText, V: s} }
func IntValue(i int64) Value { return Value{Type: TypeInt, V: i} }
func FloatValue(f float64) Value { return Value{Type: TypeFloat, V: f} }
func BoolValue(b bool) Value { return Value{Type: TypeBool, V: b} }
func DateValue(t time.Time) Value { return Value{Type: TypeDate, V: t} }
func DateTimeValue(t time.Time) Value { return Value{Type: TypeDateTime, V: t} }

func (v Value) IsNull() bool { return v.Type == TypeNull }

func (v Value) AsText() string {
	switch v.Type {
	case TypeText:
		return v.V.(string)
	case TypeInt:
		return strconv.FormatInt(v.V.(int64), 10)
	case TypeFloat:
		return strconv.FormatFloat(v.V.(float64), 'f', -1, 64)
	case TypeBool:
		if v.V.(bool) {
			return "true"
		}
		return "false"
	case TypeDate:
		return v.V.(time.Time).Format("2006-01-02")
	case TypeDateTime:
		return v.V.(time.Time).Format("2006-01-02 15:04:05")
	default:
		return "NULL"
	}
}

func (v Value) AsNumber() (float64, bool) {
	switch v.Type {
	case TypeInt:
		return float64(v.V.(int64)), true
	case TypeFloat:
		return v.V.(float64), true
	case TypeBool:
		if v.V.(bool) {
			return 1.0, true
		}
		return 0.0, true
	case TypeText:
		f, err := strconv.ParseFloat(v.V.(string), 64)
		if err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func (v Value) AsBool() bool {
	switch v.Type {
	case TypeBool:
		return v.V.(bool)
	case TypeInt:
		return v.V.(int64) != 0
	case TypeFloat:
		return v.V.(float64) != 0
	case TypeText:
		s := v.V.(string)
		return s != "" && s != "0" && s != "false" && s != "FALSE"
	default:
		return false
	}
}

func (v Value) String() string {
	if v.IsNull() {
		return "NULL"
	}
	return fmt.Sprintf("%v", v.V)
}

func PromoteTypes(a, b ValueType) ValueType {
	if a == TypeNull {
		return b
	}
	if b == TypeNull {
		return a
	}
	if a == b {
		return a
	}
	rank := map[ValueType]int{
		TypeBool:     1,
		TypeInt:      2,
		TypeFloat:    3,
		TypeDate:     4,
		TypeDateTime: 5,
		TypeText:     6,
	}
	if rank[a] > rank[b] {
		return a
	}
	return b
}

func InferValueType(s string) Value {
	s = strings.TrimSpace(s)
	if s == "" {
		return NullValue()
	}
	upper := strings.ToUpper(s)
	if upper == "NULL" {
		return NullValue()
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return BoolValue(b)
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(i)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue(f)
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02-01-2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if layout == "2006-01-02" || layout == "2006/01/02" || layout == "01/02/2006" || layout == "02-01-2006" {
				return DateValue(t)
			}
			return DateTimeValue(t)
		}
	}
	return TextValue(s)
}
