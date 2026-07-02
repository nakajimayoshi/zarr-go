package zarr_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/nakajimayoshi/zarr-go/v1"
)

// jsonEqual reports whether two JSON documents are structurally equal, ignoring
// whitespace and key ordering.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("invalid JSON a: %v (%s)", err, a)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("invalid JSON b: %v (%s)", err, b)
	}
	return reflect.DeepEqual(av, bv)
}

func Test_V1_SpecExample(t *testing.T) {
	src := []byte(`{
		"chunks": [1000, 1000],
		"compression": "blosc",
		"compression_opts": {"clevel": 5, "cname": "lz4", "shuffle": 1},
		"dtype": "<f8",
		"fill_value": null,
		"order": "C",
		"shape": [10000, 10000],
		"zarr_format": 1
	}`)

	md, err := zarr.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if md.Version != zarr.V1 {
		t.Errorf("version: got %s, want v1", md.Version)
	}
	if md.DataType != "<f8" || md.Order != zarr.OrderRowMajor {
		t.Errorf("dtype/order: got %q/%q", md.DataType, md.Order)
	}
	if !reflect.DeepEqual(md.ChunkShape, []int{1000, 1000}) {
		t.Errorf("chunk shape: got %v", md.ChunkShape)
	}
	if len(md.Codecs) != 1 || md.Codecs[0].Name != "blosc" {
		t.Fatalf("codecs: got %+v", md.Codecs)
	}
	if cfg := md.Codecs[0].Config.(map[string]any); cfg["cname"] != "lz4" {
		t.Errorf("codec config: got %v", cfg)
	}

	out, err := json.Marshal(md)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !jsonEqual(t, src, out) {
		t.Errorf("v1 round trip mismatch\n src: %s\n out: %s", src, out)
	}
}

func Test_V2_SpecExample(t *testing.T) {
	src := []byte(`{
		"chunks": [1000, 1000],
		"compressor": {"id": "blosc", "cname": "lz4", "clevel": 5, "shuffle": 1},
		"dtype": "<f8",
		"fill_value": "NaN",
		"filters": [{"id": "delta", "dtype": "<f8", "astype": "<f4"}],
		"order": "C",
		"shape": [10000, 10000],
		"zarr_format": 2
	}`)

	md, err := zarr.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if md.Version != zarr.V2 {
		t.Errorf("version: got %s, want v2", md.Version)
	}
	// filters + compressor collapse into the codec pipeline, in order.
	if len(md.Codecs) != 2 || md.Codecs[0].Name != "delta" || md.Codecs[1].Name != "blosc" {
		t.Fatalf("codecs: got %+v", md.Codecs)
	}
	if f, ok := md.FillValue.(float64); !ok || !math.IsNaN(f) {
		t.Errorf("fill_value: got %v (%T), want NaN", md.FillValue, md.FillValue)
	}

	out, err := json.Marshal(md)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !jsonEqual(t, src, out) {
		t.Errorf("v2 round trip mismatch\n src: %s\n out: %s", src, out)
	}
}

func Test_V3_SpecExample(t *testing.T) {
	src := []byte(`{
		"zarr_format": 3,
		"node_type": "array",
		"shape": [10000, 1000],
		"dimension_names": ["rows", "columns"],
		"data_type": "float64",
		"chunk_grid": {"name": "regular", "configuration": {"chunk_shape": [1000, 100]}},
		"chunk_key_encoding": {"name": "default", "configuration": {"separator": "/"}},
		"codecs": [{"name": "bytes", "configuration": {"endian": "little"}}],
		"fill_value": "NaN",
		"attributes": {"foo": 42, "bar": "apples", "baz": [1, 2, 3, 4]}
	}`)

	md, err := zarr.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if md.Version != zarr.V3 {
		t.Errorf("version: got %s, want v3", md.Version)
	}
	if md.DataType != "float64" {
		t.Errorf("data_type: got %q", md.DataType)
	}
	if !reflect.DeepEqual(md.ChunkShape, []int{1000, 100}) {
		t.Errorf("chunk shape: got %v", md.ChunkShape)
	}
	if md.Separator != "/" {
		t.Errorf("separator: got %q", md.Separator)
	}
	if !reflect.DeepEqual(md.DimNames, []string{"rows", "columns"}) {
		t.Errorf("dim names: got %v", md.DimNames)
	}
	if len(md.Codecs) != 1 || md.Codecs[0].Name != "bytes" {
		t.Fatalf("codecs: got %+v", md.Codecs)
	}
	if md.Attributes["bar"] != "apples" {
		t.Errorf("attributes: got %v", md.Attributes)
	}

	out, err := json.Marshal(md)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !jsonEqual(t, src, out) {
		t.Errorf("v3 round trip mismatch\n src: %s\n out: %s", src, out)
	}
}

func Test_FillValue_NonFinite_AllVersions(t *testing.T) {
	versions := []zarr.Version{zarr.V1, zarr.V2, zarr.V3}
	cases := []struct {
		name string
		fill any
		want string
	}{
		{"nan", math.NaN(), `"NaN"`},
		{"posinf", math.Inf(1), `"Infinity"`},
		{"neginf", math.Inf(-1), `"-Infinity"`},
		{"number", 3.5, `3.5`},
		{"null", nil, `null`},
	}
	for _, v := range versions {
		for _, tc := range cases {
			t.Run(v.String()+"/"+tc.name, func(t *testing.T) {
				md := zarr.Metadata{
					Version: v, Shape: []int{2}, ChunkShape: []int{2},
					DataType: "float64", Order: zarr.OrderRowMajor, FillValue: tc.fill,
					Codecs: []zarr.Codec{{Name: "bytes"}},
				}
				out, err := json.Marshal(md)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				var probe struct {
					FillValue json.RawMessage `json:"fill_value"`
				}
				if err := json.Unmarshal(out, &probe); err != nil {
					t.Fatalf("probe: %v", err)
				}
				if string(probe.FillValue) != tc.want {
					t.Errorf("fill_value: got %s, want %s", probe.FillValue, tc.want)
				}
			})
		}
	}
}

func Test_Groups(t *testing.T) {
	// v2 group.
	out, err := json.Marshal(zarr.Group{Version: zarr.V2})
	if err != nil {
		t.Fatalf("v2 group marshal: %v", err)
	}
	if !jsonEqual(t, out, []byte(`{"zarr_format":2}`)) {
		t.Errorf("v2 group: got %s", out)
	}

	// v3 group with attributes round-trips.
	src := []byte(`{"zarr_format":3,"node_type":"group","attributes":{"spam":"ham"}}`)
	g, err := zarr.ParseGroup(src)
	if err != nil {
		t.Fatalf("v3 group parse: %v", err)
	}
	if g.Version != zarr.V3 || g.Attributes["spam"] != "ham" {
		t.Errorf("v3 group: got %+v", g)
	}
	out, err = json.Marshal(g)
	if err != nil {
		t.Fatalf("v3 group marshal: %v", err)
	}
	if !jsonEqual(t, src, out) {
		t.Errorf("v3 group round trip: got %s", out)
	}
}

func Test_Errors(t *testing.T) {
	// Unset version.
	if _, err := json.Marshal(zarr.Metadata{Shape: []int{1}}); err == nil {
		t.Error("expected error marshaling metadata with unset version")
	}
	// Invalid order for v1.
	if _, err := json.Marshal(zarr.Metadata{Version: zarr.V1, Order: 'Z'}); err == nil {
		t.Error("expected error for invalid v1 order")
	}
	// v3 with no codecs.
	if _, err := json.Marshal(zarr.Metadata{Version: zarr.V3, Shape: []int{1}, ChunkShape: []int{1}}); err == nil {
		t.Error("expected error for v3 with no codecs")
	}
	// Structured dtype rejected.
	err := json.Unmarshal([]byte(`{"zarr_format":2,"shape":[1],"chunks":[1],"dtype":["<i4","<f8"],"order":"C"}`), &zarr.Metadata{})
	if err == nil {
		t.Error("expected error for structured dtype")
	}
	// Parsing a group as an array.
	if _, err := zarr.Parse([]byte(`{"zarr_format":3,"node_type":"group"}`)); err == nil {
		t.Error("expected error parsing group as array")
	}
}
