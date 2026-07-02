// Package zarr reads and writes Zarr array/group metadata for storage
// specification versions 1, 2 and 3.
//
// A single Metadata type models an array across all three versions; the
// Version field selects the on-the-wire encoding. The per-version JSON shapes
// (key names, codec object layout, chunk-grid nesting) are an implementation
// detail — the public surface is just Metadata, Group and Codec.
package zarr

//
//import (
//	"encoding/json"
//	"fmt"
//	"slices"
//	"unicode"
//
//	"github.com/nakajimayoshi/zarr-go/v1/internal/zcommon"
//)
//
//// Version is the value of the zarr_format metadata key.
//type Version int
//
//const (
//	V2 Version = 2
//	V3 Version = 3
//)
//
//func (v Version) String() string {
//	switch v {
//	case V2, V3:
//		return fmt.Sprintf("v%d", int(v))
//	default:
//		return fmt.Sprintf("Version(%d)", int(v))
//	}
//}
//
//// Order is the in-chunk byte layout, used by v1 and v2 ('C' row-major, 'F'
//// column-major). v3 expresses layout through a transpose codec instead.
//type Order = zcommon.Order
//
//const (
//	OrderRowMajor    = zcommon.OrderRowMajor
//	OrderColumnMajor = zcommon.OrderColumnMajor
//)
//
//// Convenience fill-value sentinels for the non-finite floats.
//var (
//	FillValueNaN    = zcommon.FillValueNaN
//	FillValueInf    = zcommon.FillValueInf
//	FillValueNegInf = zcommon.FillValueNegInf
//)
//
//// Codec is a version-agnostic codec description. Name is the codec identifier
//// (v2 "id" / v3 "name"); Config holds its parameters (a map for v2/v3, or any
//// scalar for a v1 compression_opts value). Config may be nil.
//type Codec struct {
//	Name   string
//	Config any
//}
//
//// Metadata is the metadata for a single Zarr array, normalized across versions.
////
//// Which fields are meaningful depends on Version: Order applies to v1/v2 only;
//// Attributes/DimNames apply to v3 (v2 stores attributes in a separate .zattrs
//// resource); Codecs is the ordered codec pipeline (see the per-version notes on
//// MarshalJSON for how it maps to v1 compression / v2 compressor+filters).
//type Metadata struct {
//	Version    Version
//	Shape      []int
//	ChunkShape []int
//	DataType   string
//	FillValue  any
//	Order      Order
//	Codecs     []Codec
//	Attributes map[string]any
//	Separator  string
//	DimNames   []string
//}
//
//// Group is the metadata for a Zarr group (v2/v3; v1 has no group concept).
//type Group struct {
//	Version    Version
//	Attributes map[string]any
//}
//
//// ---- public entry points ---------------------------------------------------
//
//var supportedVersions = []Version{V2, V3}
//
//// DetectFormat reads the zarr_format key from any Zarr metadata document.
//func DetectFormat(data []byte) (Version, error) {
//	var probe struct {
//		Format *int `json:"zarr_format"`
//	}
//	if err := json.Unmarshal(data, &probe); err != nil {
//		return 0, err
//	}
//	if probe.Format == nil {
//		return 0, fmt.Errorf("zarr: metadata is missing the zarr_format key")
//	}
//	v := Version(*probe.Format)
//	if !slices.Contains(supportedVersions, v) {
//		return 0, fmt.Errorf("zarr: unsupported zarr_format %d", *probe.Format)
//	}
//	return v, nil
//}
//
//// Detect reports the version of a metadata document and whether it describes a
//// group (rather than an array).
//func Detect(data []byte) (version Version, isGroup bool, err error) {
//	v, err := DetectFormat(data)
//	if err != nil {
//		return 0, false, err
//	}
//	switch v {
//	case V2:
//		var probe struct {
//			Shape json.RawMessage `json:"shape"`
//		}
//		if err := json.Unmarshal(data, &probe); err != nil {
//			return 0, false, err
//		}
//		return V2, len(probe.Shape) == 0, nil
//	case V3:
//		var probe struct {
//			NodeType string `json:"node_type"`
//		}
//		if err := json.Unmarshal(data, &probe); err != nil {
//			return 0, false, err
//		}
//		return V3, probe.NodeType == "group", nil
//	default:
//		return v, false, nil // v1 has no groups
//	}
//}
//
//// Parse decodes array metadata of any version. It returns an error if the
//// document is a group (use ParseGroup).
//func Parse(data []byte) (Metadata, error) {
//	var m Metadata
//	err := m.UnmarshalJSON(data)
//	return m, err
//}
//
//// ParseGroup decodes group metadata of any version.
//func ParseGroup(data []byte) (Group, error) {
//	var g Group
//	err := g.UnmarshalJSON(data)
//	return g, err
//}
//
//// ---- Metadata JSON ----------------------------------------------------------
//
//func (m Metadata) MarshalJSON() ([]byte, error) {
//	switch m.Version {
//	case V2:
//		return m.marshalV2()
//	case V3:
//		return m.marshalV3()
//	case 0:
//		return nil, fmt.Errorf("zarr: Metadata.Version is unset")
//	default:
//		return nil, fmt.Errorf("zarr: unsupported version %d", m.Version)
//	}
//}
//
//func (m *Metadata) UnmarshalJSON(data []byte) error {
//	v, err := DetectFormat(data)
//	if err != nil {
//		return err
//	}
//	switch v {
//	case V2:
//		return m.unmarshalV2(data)
//	case V3:
//		return m.unmarshalV3(data)
//	default:
//		return fmt.Errorf("zarr: unsupported version %d", v)
//	}
//}
//
//// ---- v2 ---------------------------------------------------------------------
//
//// v2codec is a v2 compressor/filter: {"id": ..., ...params}.
//type v2codec struct {
//	ID     string
//	Config map[string]any
//}
//
//func (c v2codec) MarshalJSON() ([]byte, error) {
//	if c.ID == "" {
//		return nil, fmt.Errorf(`zarr: v2 codec id must not be empty`)
//	}
//	obj := make(map[string]any, len(c.Config)+1)
//	for k, v := range c.Config {
//		obj[k] = v
//	}
//	obj["id"] = c.ID
//	return json.Marshal(obj)
//}
//
//func (c *v2codec) UnmarshalJSON(data []byte) error {
//	var obj map[string]any
//	if err := json.Unmarshal(data, &obj); err != nil {
//		return err
//	}
//	id, ok := obj["id"].(string)
//	if !ok || id == "" {
//		return fmt.Errorf(`zarr: v2 codec requires a string "id"`)
//	}
//	delete(obj, "id")
//	if len(obj) == 0 {
//		obj = nil
//	}
//	*c = v2codec{ID: id, Config: obj}
//	return nil
//}
//
//func codecToV2(c Codec) v2codec {
//	cfg, _ := c.Config.(map[string]any)
//	return v2codec{ID: c.Name, Config: cfg}
//}
//
//func codecFromV2(c v2codec) Codec {
//	var cfg any
//	if c.Config != nil {
//		cfg = c.Config
//	}
//	return Codec{Name: c.ID, Config: cfg}
//}
//
//type v2Wire struct {
//	Chunks             []int         `json:"chunks"`
//	Compressor         *v2codec      `json:"compressor"`
//	DType              string        `json:"dtype"`
//	FillValue          any           `json:"fill_value"`
//	Filters            []v2codec     `json:"filters"`
//	Order              zcommon.Order `json:"order"`
//	Shape              []int         `json:"shape"`
//	Format             int           `json:"zarr_format"`
//	DimensionSeparator string        `json:"dimension_separator,omitempty"`
//}
//
//func (m Metadata) marshalV2() ([]byte, error) {
//	if !m.Order.Valid() {
//		return nil, fmt.Errorf("zarr: v2 order must be C or F, got %q", m.Order)
//	}
//	// The final codec is treated as the compressor; any preceding codecs are
//	// filters. (A v2 array without a compressor is not distinguishable in this
//	// normalized form and will re-encode its last codec as the compressor.)
//	var compressor *v2codec
//	var filters []v2codec
//	if n := len(m.Codecs); n > 0 {
//		last := codecToV2(m.Codecs[n-1])
//		compressor = &last
//		for _, c := range m.Codecs[:n-1] {
//			filters = append(filters, codecToV2(c))
//		}
//	}
//	return json.Marshal(v2Wire{
//		Chunks:             m.ChunkShape,
//		Compressor:         compressor,
//		DType:              m.DataType,
//		FillValue:          zcommon.EncodeAnyFill(m.FillValue),
//		Filters:            filters,
//		Order:              m.Order,
//		Shape:              m.Shape,
//		Format:             int(V2),
//		DimensionSeparator: m.Separator,
//	})
//}
//
//func (m *Metadata) unmarshalV2(data []byte) error {
//	var w struct {
//		Chunks             []int           `json:"chunks"`
//		Compressor         *v2codec        `json:"compressor"`
//		DType              json.RawMessage `json:"dtype"`
//		FillValue          json.RawMessage `json:"fill_value"`
//		Filters            []v2codec       `json:"filters"`
//		Order              zcommon.Order   `json:"order"`
//		Shape              json.RawMessage `json:"shape"`
//		DimensionSeparator string          `json:"dimension_separator"`
//	}
//	if err := json.Unmarshal(data, &w); err != nil {
//		return err
//	}
//	if len(w.Shape) == 0 {
//		return fmt.Errorf("zarr: v2 document has no shape; it may be a group (use ParseGroup)")
//	}
//	var shape []int
//	if err := json.Unmarshal(w.Shape, &shape); err != nil {
//		return err
//	}
//	if !w.Order.Valid() {
//		return fmt.Errorf("zarr: v2 order is required and must be C or F")
//	}
//	dtype, err := scalarDType(w.DType)
//	if err != nil {
//		return err
//	}
//	fill, err := zcommon.DecodeAnyFill(w.FillValue)
//	if err != nil {
//		return err
//	}
//	var codecs []Codec
//	for _, f := range w.Filters {
//		codecs = append(codecs, codecFromV2(f))
//	}
//	if w.Compressor != nil {
//		codecs = append(codecs, codecFromV2(*w.Compressor))
//	}
//	*m = Metadata{
//		Version:    V2,
//		Shape:      shape,
//		ChunkShape: w.Chunks,
//		DataType:   dtype,
//		FillValue:  fill,
//		Order:      w.Order,
//		Codecs:     codecs,
//		Separator:  w.DimensionSeparator,
//	}
//	return nil
//}
//
//// ---- v3 ---------------------------------------------------------------------
//
//type v3codec struct {
//	Name          string         `json:"name"`
//	Configuration map[string]any `json:"configuration,omitempty"`
//}
//
//func codecToV3(c Codec) (v3codec, error) {
//	if c.Name == "" {
//		return v3codec{}, fmt.Errorf("zarr: v3 codec name must not be empty")
//	}
//	cfg, _ := c.Config.(map[string]any)
//	return v3codec{Name: c.Name, Configuration: cfg}, nil
//}
//
//func codecFromV3(c v3codec) Codec {
//	var cfg any
//	if c.Configuration != nil {
//		cfg = c.Configuration
//	}
//	return Codec{Name: c.Name, Config: cfg}
//}
//
//type v3gridConfig struct {
//	ChunkShape []int `json:"chunk_shape"`
//}
//
//type v3chunkGrid struct {
//	Name          string       `json:"name"`
//	Configuration v3gridConfig `json:"configuration"`
//}
//
//type v3sepConfig struct {
//	Separator string `json:"separator"`
//}
//
//type v3chunkKey struct {
//	Name          string       `json:"name"`
//	Configuration *v3sepConfig `json:"configuration,omitempty"`
//}
//
//type v3Wire struct {
//	Format           int            `json:"zarr_format"`
//	NodeType         string         `json:"node_type"`
//	Shape            []int          `json:"shape"`
//	DataType         string         `json:"data_type"`
//	ChunkGrid        v3chunkGrid    `json:"chunk_grid"`
//	ChunkKeyEncoding v3chunkKey     `json:"chunk_key_encoding"`
//	FillValue        any            `json:"fill_value"`
//	Codecs           []v3codec      `json:"codecs"`
//	Attributes       map[string]any `json:"attributes,omitempty"`
//	DimensionNames   []string       `json:"dimension_names,omitempty"`
//}
//
//func (m Metadata) marshalV3() ([]byte, error) {
//	if len(m.Codecs) == 0 {
//		return nil, fmt.Errorf("zarr: v3 requires at least one codec")
//	}
//	if len(m.DimNames) != 0 && len(m.DimNames) != len(m.Shape) {
//		return nil, fmt.Errorf("zarr: dimension_names length %d must match shape rank %d",
//			len(m.DimNames), len(m.Shape))
//	}
//	codecs := make([]v3codec, 0, len(m.Codecs))
//	for _, c := range m.Codecs {
//		wc, err := codecToV3(c)
//		if err != nil {
//			return nil, err
//		}
//		codecs = append(codecs, wc)
//	}
//	key := v3chunkKey{Name: "default"}
//	if m.Separator != "" {
//		key.Configuration = &v3sepConfig{Separator: m.Separator}
//	}
//	return json.Marshal(v3Wire{
//		Format:           int(V3),
//		NodeType:         "array",
//		Shape:            m.Shape,
//		DataType:         m.DataType,
//		ChunkGrid:        v3chunkGrid{Name: "regular", Configuration: v3gridConfig{ChunkShape: m.ChunkShape}},
//		ChunkKeyEncoding: key,
//		FillValue:        zcommon.EncodeAnyFill(m.FillValue),
//		Codecs:           codecs,
//		Attributes:       m.Attributes,
//		DimensionNames:   m.DimNames,
//	})
//}
//
//func (m *Metadata) unmarshalV3(data []byte) error {
//	var w struct {
//		NodeType         string          `json:"node_type"`
//		Shape            []int           `json:"shape"`
//		DataType         json.RawMessage `json:"data_type"`
//		ChunkGrid        v3chunkGrid     `json:"chunk_grid"`
//		ChunkKeyEncoding v3chunkKey      `json:"chunk_key_encoding"`
//		FillValue        json.RawMessage `json:"fill_value"`
//		Codecs           []v3codec       `json:"codecs"`
//		Attributes       map[string]any  `json:"attributes"`
//		DimensionNames   []string        `json:"dimension_names"`
//	}
//	if err := json.Unmarshal(data, &w); err != nil {
//		return err
//	}
//	if w.NodeType != "array" {
//		return fmt.Errorf("zarr: v3 node_type %q is not \"array\" (use ParseGroup for groups)", w.NodeType)
//	}
//	if len(w.Codecs) == 0 {
//		return fmt.Errorf("zarr: v3 requires at least one codec")
//	}
//	dtype, err := scalarDataTypeV3(w.DataType)
//	if err != nil {
//		return err
//	}
//	fill, err := zcommon.DecodeAnyFill(w.FillValue)
//	if err != nil {
//		return err
//	}
//	codecs := make([]Codec, 0, len(w.Codecs))
//	for _, c := range w.Codecs {
//		codecs = append(codecs, codecFromV3(c))
//	}
//	sep := ""
//	if w.ChunkKeyEncoding.Configuration != nil {
//		sep = w.ChunkKeyEncoding.Configuration.Separator
//	}
//	*m = Metadata{
//		Version:    V3,
//		Shape:      w.Shape,
//		ChunkShape: w.ChunkGrid.Configuration.ChunkShape,
//		DataType:   dtype,
//		FillValue:  fill,
//		Codecs:     codecs,
//		Attributes: w.Attributes,
//		Separator:  sep,
//		DimNames:   w.DimensionNames,
//	}
//	return nil
//}
//
//// ---- Group JSON -------------------------------------------------------------
//
//func (g Group) MarshalJSON() ([]byte, error) {
//	switch g.Version {
//	case V2:
//		// v2 group attributes live in a separate .zattrs resource.
//		return json.Marshal(struct {
//			Format int `json:"zarr_format"`
//		}{int(V2)})
//	case V3:
//		return json.Marshal(struct {
//			Format     int            `json:"zarr_format"`
//			NodeType   string         `json:"node_type"`
//			Attributes map[string]any `json:"attributes,omitempty"`
//		}{int(V3), "group", g.Attributes})
//	case 0:
//		return nil, fmt.Errorf("zarr: Group.Version is unset")
//	default:
//		return nil, fmt.Errorf("zarr: unsupported version %d", g.Version)
//	}
//}
//
//func (g *Group) UnmarshalJSON(data []byte) error {
//	v, err := DetectFormat(data)
//	if err != nil {
//		return err
//	}
//	switch v {
//	case V2:
//		var probe struct {
//			Shape json.RawMessage `json:"shape"`
//		}
//		if err := json.Unmarshal(data, &probe); err != nil {
//			return err
//		}
//		if len(probe.Shape) != 0 {
//			return fmt.Errorf("zarr: v2 document has a shape; it is an array (use Parse)")
//		}
//		*g = Group{Version: V2}
//		return nil
//	case V3:
//		var probe struct {
//			NodeType   string         `json:"node_type"`
//			Attributes map[string]any `json:"attributes"`
//		}
//		if err := json.Unmarshal(data, &probe); err != nil {
//			return err
//		}
//		if probe.NodeType != "group" {
//			return fmt.Errorf("zarr: v3 node_type %q is not \"group\" (use Parse for arrays)", probe.NodeType)
//		}
//		*g = Group{Version: V3, Attributes: probe.Attributes}
//		return nil
//	default:
//		return fmt.Errorf("zarr: version %s has no group metadata", v)
//	}
//}
//
//// ---- helpers ----------------------------------------------------------------
//
//// scalarDType decodes a v1/v2 dtype, which may be a bare typestr or a list. The
//// unified model represents only scalar dtypes; a structured (list) dtype is an
//// error.
//func scalarDType(raw json.RawMessage) (string, error) {
//	list, err := zcommon.DecodeDType(raw)
//	if err != nil {
//		return "", err
//	}
//	switch len(list) {
//	case 0:
//		return "", nil
//	case 1:
//		return list[0], nil
//	default:
//		return "", fmt.Errorf("zarr: structured (list) dtypes are not supported by the unified model (%d fields)", len(list))
//	}
//}
//
//// scalarDataTypeV3 decodes a v3 data_type, which is a core string or an
//// extension {name, configuration} object. Only the name is retained.
//func scalarDataTypeV3(raw json.RawMessage) (string, error) {
//	if len(raw) == 0 {
//		return "", nil
//	}
//	var s string
//	if err := json.Unmarshal(raw, &s); err == nil {
//		return s, nil
//	}
//	var obj struct {
//		Name string `json:"name"`
//	}
//	if err := json.Unmarshal(raw, &obj); err != nil {
//		return "", fmt.Errorf("zarr: data_type must be a string or {name, configuration}: %w", err)
//	}
//	return obj.Name, nil
//}
//
//// ---- store ------------------------------------------------------------------
//
//// Key is a store key: an ASCII string.
//type Key string
//
//func (k Key) String() string { return string(k) }
//
//func isAscii(s string) bool {
//	for i := 0; i < len(s); i++ {
//		if s[i] > unicode.MaxASCII {
//			return false
//		}
//	}
//	return true
//}
//
//func keyFromString(str string) (Key, error) {
//	if !isAscii(str) {
//		return Key(""), fmt.Errorf("the provided string must only contain ASCII characters")
//	}
//	return Key(str), nil
//}
//
//// Array is an in-memory key/value store implementing the Zarr store interface.
//type Array struct {
//	inner map[Key][]byte
//}
//
//func (a *Array) Read(key string) []byte {
//	return a.inner[Key(key)]
//}
