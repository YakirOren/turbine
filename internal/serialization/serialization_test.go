package serialization

import "testing"

func TestSerializerRoundTrip(t *testing.T) {
	encoded, err := EncodeJSON("hello")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeJSON[string](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello" {
		t.Fatalf("expected 'hello', got '%s'", decoded)
	}
}

func TestSerializerNil(t *testing.T) {
	encoded, err := EncodeJSON[*string](nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeJSON[*string](encoded)
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
	encoded, err := EncodeJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeJSON[testData](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != input.Name || decoded.Value != input.Value {
		t.Fatalf("expected %+v, got %+v", input, decoded)
	}
}
