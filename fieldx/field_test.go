package fieldx

import (
	"encoding/json"
	"testing"
)

type patch struct {
	Name    Field[string]  `json:"name"`
	Count   Field[int]     `json:"count"`
	Pointer Field[*string] `json:"pointer"`
	Nested  Field[struct {
		Enabled bool `json:"enabled"`
	}] `json:"nested"`
}

func TestUnmarshalStates(t *testing.T) {
	var got patch
	if err := json.Unmarshal([]byte(`{"count":0,"pointer":null,"nested":{"enabled":false}}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name.IsSet() {
		t.Fatal("omitted field marked as set")
	}
	if value, ok := got.Count.Value(); !ok || value != 0 {
		t.Fatalf("zero value not preserved: %d %v", value, ok)
	}
	if !got.Pointer.IsNull() {
		t.Fatal("null pointer field not preserved")
	}
	if value, ok := got.Nested.Value(); !ok || value.Enabled {
		t.Fatalf("nested value not preserved: %#v %v", value, ok)
	}
}

func TestConstructorsAndMarshal(t *testing.T) {
	value := "value"
	input := patch{
		Name:    Value(""),
		Count:   Value(0),
		Pointer: Value(&value),
		Nested: Null[struct {
			Enabled bool `json:"enabled"`
		}](),
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"name":"","count":0,"pointer":"value","nested":null}`
	if string(data) != want {
		t.Fatalf("got %s, want %s", data, want)
	}
}

func TestUnsetMarshalAndOmitemptyLimitation(t *testing.T) {
	data, err := json.Marshal(struct {
		Value Field[string] `json:"value,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"value":null}` {
		t.Fatalf("unexpected unset encoding: %s", data)
	}
}

func TestUnsetOmitsWithOmitzero(t *testing.T) {
	data, err := json.Marshal(struct {
		Value Field[string] `json:"value,omitzero"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{}` {
		t.Fatalf("unexpected unset encoding: %s", data)
	}
}

func TestReuseDestination(t *testing.T) {
	field := Value("old")
	if err := json.Unmarshal([]byte("null"), &field); err != nil {
		t.Fatal(err)
	}
	if !field.IsNull() {
		t.Fatal("destination did not reset to null")
	}
	if err := json.Unmarshal([]byte(`"new"`), &field); err != nil {
		t.Fatal(err)
	}
	if value, ok := field.Value(); !ok || value != "new" {
		t.Fatalf("unexpected reused value: %q %v", value, ok)
	}
	field.Reset()
	if field.IsSet() {
		t.Fatal("reset field should be unset")
	}
}

func TestReusedStructMustBeResetForOmittedFields(t *testing.T) {
	got := patch{Name: Value("old")}
	if err := json.Unmarshal([]byte(`{}`), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Name.IsSet() {
		t.Fatal("encoding/json unexpectedly visited an omitted field")
	}
	got = patch{}
	if err := json.Unmarshal([]byte(`{}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name.IsSet() {
		t.Fatal("zeroed destination retained state")
	}
}

func TestMustValue(t *testing.T) {
	if Value(42).MustValue() != 42 {
		t.Fatal("unexpected value")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Null[int]().MustValue()
}
