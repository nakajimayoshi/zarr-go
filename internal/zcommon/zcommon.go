// Package zcommon holds the encoding primitives shared by the versioned Zarr
// metadata packages: byte order, NumPy-style dtypes, and fill-value codecs.
// It has no dependencies on the versioned packages, so it can be imported by
// all of them without creating an import cycle.
package zcommon

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Order is the in-chunk byte layout: 'C' (row-major, last dimension varies
// fastest) or 'F' (column-major, first dimension varies fastest). It encodes
// as the JSON string "C" or "F".
type Order byte

const (
	OrderRowMajor    Order = 'C'
	OrderColumnMajor Order = 'F'
)

// Valid reports whether o is one of the two defined orders.
func (o Order) Valid() bool { return o == OrderRowMajor || o == OrderColumnMajor }

func (o Order) String() string {
	if o == 0 {
		return ""
	}
	return string(rune(o))
}

func (o Order) MarshalJSON() ([]byte, error) {
	if !o.Valid() {
		return nil, fmt.Errorf("zarr: order must be %q or %q, got %q",
			OrderRowMajor, OrderColumnMajor, o)
	}
	return json.Marshal(o.String())
}

func (o *Order) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	ord := Order(0)
	if len(s) == 1 {
		ord = Order(s[0])
	}
	if !ord.Valid() {
		return fmt.Errorf("zarr: order must be %q or %q, got %q",
			OrderRowMajor, OrderColumnMajor, s)
	}
	*o = ord
	return nil
}

// EncodeDType renders a NumPy-style dtype for the wire: a single entry becomes
// a bare typestr string (e.g. "<f8"), a multi-entry dtype becomes a JSON list.
//
// Only scalar typestrs and lists of typestrs are modelled; the full NumPy
// structured "descr" grammar (nested [name, type] / [name, type, shape]
// records) is out of scope.
func EncodeDType(dtype []string) (any, error) {
	switch len(dtype) {
	case 0:
		return nil, fmt.Errorf("zarr: dtype must contain at least one entry")
	case 1:
		return dtype[0], nil
	default:
		return dtype, nil
	}
}

// DecodeDType parses a dtype that is either a bare string or a list of strings.
func DecodeDType(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("zarr: dtype must be a string or a list of strings: %w", err)
	}
	return list, nil
}

// Sentinel fill values for the non-finite floats that JSON cannot represent
// natively. On the wire these encode as the strings "NaN", "Infinity" and
// "-Infinity". They are convenience pointers only — the codecs detect
// non-finite values by inspection, so a hand-built *float64 works identically.
var (
	fillNaN    = math.NaN()
	fillPosInf = math.Inf(1)
	fillNegInf = math.Inf(-1)

	FillValueNaN    = &fillNaN
	FillValueInf    = &fillPosInf
	FillValueNegInf = &fillNegInf
)

// EncodeFloatFill renders a *float64 fill value: nil -> null, non-finite ->
// string sentinel, otherwise the bare number.
func EncodeFloatFill(fv *float64) any {
	if fv == nil {
		return nil
	}
	if s := nonFiniteString(*fv); s != "" {
		return s
	}
	return *fv
}

// DecodeFloatFill parses a fill value into a *float64, accepting a JSON number,
// null, or one of the float sentinel strings.
func DecodeFloatFill(raw json.RawMessage) (*float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case nil:
		return nil, nil
	case float64:
		return &t, nil
	case string:
		f, ok := parseNonFinite(t)
		if !ok {
			return nil, fmt.Errorf(`zarr: fill_value string must be "NaN"/"Infinity"/"-Infinity", got %q`, t)
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("zarr: fill_value must be a number, null, or float sentinel string, got %T", t)
	}
}

// EncodeAnyFill renders a fill value of arbitrary scalar type (v2/v3), where
// fill_value may be a number, boolean, null, string, or array. A non-finite
// float64 (or *float64) is translated to its string sentinel; everything else
// passes through untouched.
func EncodeAnyFill(v any) any {
	switch f := v.(type) {
	case float64:
		if s := nonFiniteString(f); s != "" {
			return s
		}
	case *float64:
		return EncodeFloatFill(f)
	}
	return v
}

// DecodeAnyFill parses a v2/v3 fill_value into a native Go value: numbers as
// float64, the float sentinel strings as non-finite float64, and everything
// else (bool, null, other strings, arrays, objects) as its natural decoding.
func DecodeAnyFill(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if s, ok := v.(string); ok {
		if f, ok := parseNonFinite(s); ok {
			return f, nil
		}
	}
	return v, nil
}

func nonFiniteString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	default:
		return ""
	}
}

func parseNonFinite(s string) (float64, bool) {
	switch strings.ToLower(s) {
	case "nan":
		return math.NaN(), true
	case "infinity", "inf", "+infinity", "+inf":
		return math.Inf(1), true
	case "-infinity", "-inf":
		return math.Inf(-1), true
	default:
		return 0, false
	}
}
