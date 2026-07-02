package zarr

import (
	"cmp"
	"errors"
	"fmt"
)

// ByteOffset represents a byte offset.
type ByteOffset uint64

// ByteLength represents a byte length.
type ByteLength uint64

type byteRangeKind int

const (
	// A byte range from the start.
	//
	// If the byte length is 0, reads to the end of the value.
	// (Port note: Rust models this as FromStart(offset, Option<ByteLength>);
	// we encode None as byteLength == 0. A zero-length FromStart read is not
	// expressible, which is fine — it would be a no-op.)
	kindFromStart byteRangeKind = iota

	// A suffix byte range.
	kindSuffix
)

// ByteRange represents a byte range relative to the start or end of a byte
// sequence.
type ByteRange struct {
	kind       byteRangeKind
	offset     ByteOffset
	byteLength ByteLength
}

// NewByteRangeFromStart creates a ByteRange that begins at offset.
// A length of 0 reads to the end of the value.
func NewByteRangeFromStart(offset ByteOffset, length ByteLength) ByteRange {
	return ByteRange{
		kind:       kindFromStart,
		offset:     offset,
		byteLength: length,
	}
}

// NewByteRangeSuffix creates a ByteRange covering the final length bytes.
// A suffix of zero bytes is invalid.
func NewByteRangeSuffix(length ByteLength) (ByteRange, error) {
	if length == 0 {
		return ByteRange{}, errors.New("zarr: suffix byte range length cannot be zero")
	}
	return ByteRange{
		kind:       kindSuffix,
		byteLength: length,
	}, nil
}

func (b ByteRange) toEnd() bool {
	return b.kind == kindFromStart && b.byteLength == 0
}

// Compare orders byte ranges the same way as the Rust Ord impl:
// FromStart < Suffix; within FromStart, by offset then length, where
// "read to end" (None) sorts before any explicit length (Some).
// The None-before-Some ordering falls out of byteLength == 0 encoding None.
func (b ByteRange) Compare(other ByteRange) int {
	if b.kind != other.kind {
		return cmp.Compare(b.kind, other.kind)
	}
	if b.offset != other.offset {
		return cmp.Compare(b.offset, other.offset)
	}
	return cmp.Compare(b.byteLength, other.byteLength)
}

// String mirrors the Rust Display impl: "..", "5..", "5..7", "-2..".
func (b ByteRange) String() string {
	switch b.kind {
	case kindFromStart:
		start := ""
		if b.offset != 0 {
			start = fmt.Sprintf("%d", b.offset)
		}
		end := ""
		if !b.toEnd() {
			end = fmt.Sprintf("%d", uint64(b.offset)+uint64(b.byteLength))
		}
		return start + ".." + end
	case kindSuffix:
		return fmt.Sprintf("-%d..", b.byteLength)
	default:
		return fmt.Sprintf("ByteRange(invalid kind %d)", b.kind)
	}
}

// Start returns the inclusive start of the byte range within a value of the
// given size. The range must have been validated against size first.
func (b ByteRange) Start(size uint64) uint64 {
	switch b.kind {
	case kindSuffix:
		return size - uint64(b.byteLength)
	default: // kindFromStart
		return uint64(b.offset)
	}
}

// End returns the exclusive end of the byte range within a value of the
// given size. The range must have been validated against size first.
func (b ByteRange) End(size uint64) uint64 {
	switch b.kind {
	case kindFromStart:
		if b.toEnd() {
			return size
		}
		return uint64(b.offset) + uint64(b.byteLength)
	default: // kindSuffix
		return size
	}
}

// Length returns the length of the byte range within a value of the given
// size. The range must have been validated against size first.
func (b ByteRange) Length(size uint64) uint64 {
	if b.toEnd() {
		return size - uint64(b.offset)
	}
	return uint64(b.byteLength)
}

// validate ensures a FromStart range is valid when
// offset + length <= size (offset <= size when reading to the end), and a
// Suffix range is valid when length <= size.
func (b ByteRange) validate(size uint64) error {
	switch b.kind {
	case kindFromStart:
		// Written to avoid uint64 overflow in offset + byteLength.
		if uint64(b.offset) > size || uint64(b.byteLength) > size-uint64(b.offset) {
			return &InvalidByteRangeError{ByteRange: b, Size: size}
		}
		return nil
	case kindSuffix:
		if uint64(b.byteLength) > size {
			return &InvalidByteRangeError{ByteRange: b, Size: size}
		}
		return nil
	default:
		return fmt.Errorf("zarr: unknown or invalid byte range kind: %d", b.kind)
	}
}

// InvalidByteRangeError reports a byte range that requests bytes beyond the
// end of a value.
type InvalidByteRangeError struct {
	ByteRange ByteRange
	Size      uint64
}

func (e *InvalidByteRangeError) Error() string {
	return fmt.Sprintf("zarr: invalid byte range %s for bytes of length %d", e.ByteRange, e.Size)
}

// ExtractByteRanges extracts byte ranges from a value. Each returned slice is a copy.
func extractByteRanges(value []byte, byteRanges []ByteRange) ([][]byte, error) {
	size := uint64(len(value))
	out := make([][]byte, 0, len(byteRanges))
	for _, br := range byteRanges {
		if err := br.validate(size); err != nil {
			return nil, err
		}
		start, end := br.Start(size), br.End(size)
		segment := make([]byte, end-start)
		copy(segment, value[start:end])
		out = append(out, segment)
	}
	return out, nil
}
