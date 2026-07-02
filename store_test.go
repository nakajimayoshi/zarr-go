package zarr_test

import (
	"context"
	"testing"

	"github.com/nakajimayoshi/zarr-go/v1"
)

func TestByteRanges(t *testing.T) {
	br := zarr.NewByteRangeFromStart(1, 0)
	if br.Start(10) != 1 || br.End(10) != 10 || br.Length(10) != 9 {
		t.Fatal("FromStart(1, None)")
	}
	sfx, _ := zarr.NewByteRangeSuffix(1)
	if sfx.Start(10) != 9 || sfx.End(10) != 10 || sfx.Length(10) != 1 {
		t.Fatal("Suffix(1)")
	}
	br = zarr.NewByteRangeFromStart(1, 5)
	if br.Start(10) != 1 || br.End(10) != 6 || br.Length(10) != 5 {
		t.Fatal("FromStart(1, Some(5))")
	}
	if br.validate(6) != nil {
		t.Fatal("valid range rejected")
	}
	if br.validate(2) == nil {
		t.Fatal("invalid range accepted")
	}
	sfx5, _ := zarr.NewByteRangeSuffix(5)
	if sfx5.validate(6) != nil || sfx5.validate(2) == nil {
		t.Fatal("suffix validation")
	}
	_, err := zarr.ExtractByteRanges([]byte{1, 2, 3}, []zarr.ByteRange{zarr.NewByteRangeFromStart(1, 4)})
	if err == nil || err.Error() != "zarr: invalid byte range 1..5 for bytes of length 3" {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestCompareOrdering(t *testing.T) {
	// None (to-end) sorts before Some(length); FromStart before Suffix.
	toEnd := zarr.NewByteRangeFromStart(1, 0)
	some := zarr.NewByteRangeFromStart(1, 3)
	sfx, _ := zarr.NewByteRangeSuffix(1)
	if toEnd.Compare(some) >= 0 {
		t.Fatal("None should sort before Some")
	}
	if some.Compare(sfx) >= 0 {
		t.Fatal("FromStart should sort before Suffix")
	}
}

func TestDisplay(t *testing.T) {
	cases := map[string]zarr.ByteRange{
		"..":   zarr.NewByteRangeFromStart(0, 0),
		"5..":  zarr.NewByteRangeFromStart(5, 0),
		"5..7": zarr.NewByteRangeFromStart(5, 2),
	}
	sfx, _ := zarr.NewByteRangeSuffix(2)
	cases["-2.."] = sfx
	for want, br := range cases {
		if br.String() != want {
			t.Fatalf("got %q want %q", br.String(), want)
		}
	}
}

func TestInMemoryStore(t *testing.T) {
	ctx := context.Background()
	s := zarr.NewInMemoryStore()
	key, _ := zarr.NewStoreKey("a/b")
	if v, err := s.Get(ctx, key); v != nil || err != nil {
		t.Fatal("missing key should be (nil, nil)")
	}
	if v, err := s.GetPartial(ctx, key, zarr.NewByteRangeFromStart(0, 1)); v != nil || err != nil {
		t.Fatal("missing key partial should be (nil, nil)")
	}
	//s.data.Store(key, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	sfx, _ := zarr.NewByteRangeSuffix(5)
	out, err := s.GetPartialMany(ctx, key, []zarr.ByteRange{
		zarr.NewByteRangeFromStart(3, 3),
		zarr.NewByteRangeFromStart(4, 1),
		sfx,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]byte{{3, 4, 5}, {4}, {5, 6, 7, 8, 9}}
	for i := range want {
		if string(out[i]) != string(want[i]) {
			t.Fatalf("range %d: got %v want %v", i, out[i], want[i])
		}
	}
	if n, _ := s.KeySize(ctx, key); n != 10 {
		t.Fatalf("KeySize got %d", n)
	}
}
