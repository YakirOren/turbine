package turbine

import "testing"

func TestSerializerRoundTrip(t *testing.T) {
	encoded, err := encodeJSON("hello")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJSON[string](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello" {
		t.Fatalf("expected 'hello', got '%s'", decoded)
	}
}

func TestSerializerNil(t *testing.T) {
	encoded, err := encodeJSON[*string](nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJSON[*string](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != nil {
		t.Fatal("expected nil")
	}
}

func TestSerializerStruct(t *testing.T) {
	type testData struct {
		Name  string
		Value int
	}
	input := testData{Name: "test", Value: 42}
	encoded, err := encodeJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJSON[testData](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != input.Name || decoded.Value != input.Value {
		t.Fatalf("expected %+v, got %+v", input, decoded)
	}
}
