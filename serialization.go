package pbdbos

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type serializer[T any] interface {
	Encode(data T) (*string, error)
	Decode(data *string) (T, error)
}

type jsonSerializer[T any] struct{}

func newJSONSerializer[T any]() serializer[T] {
	return &jsonSerializer[T]{}
}

func (j *jsonSerializer[T]) Encode(data T) (*string, error) {
	if isNilValue(data) {
		return nil, nil
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}
	s := string(jsonBytes)
	return &s, nil
}

func (j *jsonSerializer[T]) Decode(data *string) (T, error) {
	if data == nil || *data == "" {
		return getNilOrZeroValue[T](), nil
	}
	var result T
	if err := json.Unmarshal([]byte(*data), &result); err != nil {
		return result, fmt.Errorf("failed to decode json data: %w", err)
	}
	return result, nil
}

func isNilValue(v any) bool {
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return true
	}
	switch val.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Interface:
		return val.IsNil()
	}
	return false
}

func getNilOrZeroValue[T any]() T {
	var result T
	resultType := reflect.TypeOf(result)
	if resultType == nil {
		return result
	}
	if resultType.Kind() == reflect.Pointer {
		return reflect.Zero(resultType).Interface().(T)
	}
	return result
}
