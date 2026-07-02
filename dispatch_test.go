package zarr_test

import (
	"testing"

	zarr "github.com/nakajimayoshi/zarr-go/v1"
)

func Test_Detect(t *testing.T) {
	cases := []struct {
		data    string
		want    zarr.Version
		isGroup bool
		err     bool
	}{
		{`{"zarr_format":1,"order":"C"}`, zarr.V1, false, false},
		{`{"zarr_format":2,"shape":[1]}`, zarr.V2, false, false},
		{`{"zarr_format":2}`, zarr.V2, true, false},
		{`{"zarr_format":3,"node_type":"array"}`, zarr.V3, false, false},
		{`{"zarr_format":3,"node_type":"group"}`, zarr.V3, true, false},
		{`{"shape":[1]}`, 0, false, true},     // missing zarr_format
		{`{"zarr_format":9}`, 0, false, true}, // unsupported
	}
	for _, tc := range cases {
		v, isGroup, err := zarr.Detect([]byte(tc.data))
		if tc.err {
			if err == nil {
				t.Errorf("%s: expected error", tc.data)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.data, err)
			continue
		}
		if v != tc.want || isGroup != tc.isGroup {
			t.Errorf("%s: got (%s, group=%v), want (%s, group=%v)", tc.data, v, isGroup, tc.want, tc.isGroup)
		}
	}
}

func Test_Parse_AllVersions(t *testing.T) {
	docs := map[zarr.Version]string{
		zarr.V1: `{"zarr_format":1,"shape":[4],"chunks":[2],"dtype":"<f8","compression":"blosc","fill_value":null,"order":"C"}`,
		zarr.V2: `{"zarr_format":2,"shape":[4],"chunks":[2],"dtype":"<f8","compressor":null,"fill_value":null,"order":"C","filters":null}`,
		zarr.V3: `{"zarr_format":3,"node_type":"array","shape":[4],"data_type":"int32",` +
			`"chunk_grid":{"name":"regular","configuration":{"chunk_shape":[2]}},` +
			`"chunk_key_encoding":{"name":"default"},"codecs":[{"name":"bytes"}],"fill_value":0}`,
	}
	for v, doc := range docs {
		md, err := zarr.Parse([]byte(doc))
		if err != nil {
			t.Errorf("%s: parse error %v", v, err)
			continue
		}
		if md.Version != v {
			t.Errorf("%s: got version %s", v, md.Version)
		}
	}
}
