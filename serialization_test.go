package pbdbos

import "testing"

func TestSerializerRoundTrip(t *testing.T) {
	s := newJSONSerializer[string]()
	encoded, err := s.Encode("hello")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello" {
		t.Fatalf("expected 'hello', got '%s'", decoded)
	}
}

func TestSerializerNil(t *testing.T) {
	s := newJSONSerializer[*string]()
	encoded, err := s.Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.Decode(encoded)
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
	s := newJSONSerializer[testData]()
	input := testData{Name: "test", Value: 42}
	encoded, err := s.Encode(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != input.Name || decoded.Value != input.Value {
		t.Fatalf("expected %+v, got %+v", input, decoded)
	}
}
