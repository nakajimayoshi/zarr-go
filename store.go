package zarr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

type StoreKey struct {
	path string
}

// NewStoreKey / Validates a key according to the following rule from the specification:
// / - a key is a Unicode string, where the final character is not a `/` character.
// / - a key must only use ASCII characters
// /
// / Additional checks (not in the specification):
// / - a key cannot be an empty string.
// / - a key which starts with '/' is invalid, and
// / - a key that contains '//' is invalid, and
func NewStoreKey(key string) (StoreKey, error) {
	if key == "" {
		return StoreKey{}, fmt.Errorf("StoreKey cannot be empty string")
	}
	if strings.HasPrefix(key, "/") {
		return StoreKey{}, fmt.Errorf("StoreKey cannot begin with /")
	}
	if strings.HasSuffix(key, "/") {
		return StoreKey{}, fmt.Errorf("StoreKey cannot end with /")
	}
	if strings.Contains(key, "//") {
		return StoreKey{}, fmt.Errorf("StoreKey cannot contain //")
	}

	if !isAscii(key) {
		return StoreKey{}, fmt.Errorf("StoreKey can only contain ASCII characters")
	}
	return StoreKey{path: key}, nil
}

func (s StoreKey) String() string {
	return s.path
}

func isAscii(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

type StorePrefix struct {
	path string
}

func (s *StorePrefix) String() string {
	return s.path
}

func (s *StorePrefix) Root() string {
	return ""
}

func (s *StorePrefix) Parent() *StorePrefix {
	panic("not implemented!")
}

func newStorePrefix(prefix string) (StorePrefix, error) {
	panic("no implemented!")
	//if _, err := NewStoreKey(prefix); err != nil {
	//	errMsg := strings.Replace(err.Error(), "StoreKey", "StorePrefix", 1)
	//	return StorePrefix(""), fmt.Errorf("%s", errMsg)
	//}
	//return StorePrefix(prefix), nil
}

type StorageError struct {
	err string
}

func (s *StorageError) Error() string {
	return s.err
}

type ReadableStore interface {
	// Get retrieves the value (bytes) associated with a given [StoreKey].
	// Returns (nil, nil) if the key is not found.
	// Returns a [StorageError] if there is an underlying storage error.
	Get(ctx context.Context, key StoreKey) ([]byte, error)
	GetPartial(ctx context.Context, key StoreKey, byteRange ByteRange) ([]byte, error)
	GetPartialMany(ctx context.Context, key StoreKey, byteRange []ByteRange) ([][]byte, error)
	KeySize(ctx context.Context, key StoreKey) (int, error)
}

type WriteableStore interface {
	// Set stores bytes at a [StoreKey].
	Set(ctx context.Context, key StoreKey, value []byte) error
	// SetPartial stores bytes from an offset and value
	SetPartial(ctx context.Context, key StoreKey, offset ByteOffset, value []byte) error
	// SetPartialMany stores bytes from a slice of [ByteOffset]
	SetPartialMany(ctx context.Context, key StoreKey, entries []SetPartialManyEntry) error
	// Delete deletes the contents of a [StoreKey].
	Delete(ctx context.Context, key StoreKey) error
	// DeleteMany deletes a list of [StoreKey]
	DeleteMany(ctx context.Context, keys []StoreKey) error
	// DeletePrefix deletes all [StoreKey] under the [StorePrefix]
	DeletePrefix(ctx context.Context, prefix StorePrefix) error
}

type ListableStore interface {
	List(ctx context.Context) []StoreKey
	ListPrefix(ctx context.Context)
}

type Store interface {
	ReadableStore
	WriteableStore
	ListableStore
}

type InMemoryStore struct {
	data sync.Map
}

func NewInMemoryStore() InMemoryStore {
	return InMemoryStore{
		data: sync.Map{},
	}
}

func (i *InMemoryStore) Get(ctx context.Context, key StoreKey) ([]byte, error) {
	value, _ := i.get(key)
	return value, nil
}

func (i *InMemoryStore) GetPartial(ctx context.Context, key StoreKey, byteRange ByteRange) ([]byte, error) {
	segments, err := i.GetPartialMany(ctx, key, []ByteRange{byteRange})
	if err != nil {
		return nil, err
	}
	if segments == nil {
		return nil, nil
	}
	if len(segments) != 1 {
		return nil, &StorageError{err: "expected one byte range"}
	}
	return segments[0], nil
}

// get retrieves from the map & casts the underlying value as []byte
func (i *InMemoryStore) get(key StoreKey) ([]byte, bool) {
	value, ok := i.data.Load(key)
	if !ok {
		return nil, false
	}
	return value.([]byte), true
}

func (i *InMemoryStore) GetPartialMany(ctx context.Context, key StoreKey, byteRanges []ByteRange) ([][]byte, error) {
	value, ok := i.get(key)
	if !ok {
		return nil, nil
	}
	return extractByteRanges(value, byteRanges)
}

func (i *InMemoryStore) KeySize(ctx context.Context, key StoreKey) (int, error) {
	value, ok := i.data.Load(key)
	if !ok {
		return 0, nil
	}
	bytes := value.([]byte)

	return len(bytes), nil
}

func (i *InMemoryStore) Set(ctx context.Context, key StoreKey, value []byte) error {
	i.data.Store(key, value)
	return nil
}

func (i *InMemoryStore) Delete(ctx context.Context, key StoreKey) error {
	i.data.Delete(key)
	return nil
}

// SetPartial stores bytes from an offset and value
func (i *InMemoryStore) SetPartial(ctx context.Context, key StoreKey, offset ByteOffset, value []byte) error {
	return i.SetPartialMany(ctx, key, []SetPartialManyEntry{
		{
			offset,
			value,
		},
	},
	)
}

type SetPartialManyEntry struct {
	Offset ByteOffset
	Value  []byte
}

// SetPartialMany stores bytes from a slice of [ByteOffset]
func (i *InMemoryStore) SetPartialMany(ctx context.Context, key StoreKey, entries []SetPartialManyEntry) error {
	var errs []error
	for _, entry := range entries {
		if err := i.SetPartial(ctx, key, entry.Offset, entry.Value); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// DeleteMany deletes a list of [StoreKey]
func (i *InMemoryStore) DeleteMany(ctx context.Context, keys []StoreKey) error {
	var errs []error
	for _, key := range keys {
		if err := i.Delete(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DeletePrefix deletes all [StoreKey] under the [StorePrefix]
func (i *InMemoryStore) DeletePrefix(ctx context.Context, prefix StorePrefix) error {
	var keys []StoreKey
	i.data.Range(func(key, _ any) bool {
		storeKey := key.(StoreKey)
		keys = append(keys, storeKey)
		return true
	})

	var errs []error
	for _, k := range keys {
		if strings.HasPrefix(k.String(), prefix.String()) {
			if err := i.Delete(ctx, k); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
