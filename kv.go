package turbine

import (
	"context"
	"fmt"

	"github.com/YakirOren/turbine/internal/serialization"
	"github.com/YakirOren/turbine/internal/sysdb"
)

// KVSet creates or overwrites a key-value pair in the global store.
func (rt *Runtime) KVSet(ctx context.Context, key string, value any) error {
	if key == "" {
		return fmt.Errorf("turbine: KV key must not be empty")
	}

	encoded, err := serialization.EncodeJSON[any](value)
	if err != nil {
		return fmt.Errorf("turbine: failed to serialize KV value: %w", err)
	}
	if encoded == nil {
		return fmt.Errorf("turbine: KV value must not be nil")
	}

	return rt.kv.SetKV(ctx, sysdb.SetKVInput{
		Key:   key,
		Value: encoded,
	})
}

// KVDelete removes a key from the global store. No-op if the key doesn't exist.
func (rt *Runtime) KVDelete(ctx context.Context, key string) error {
	return rt.kv.DeleteKV(ctx, sysdb.DeleteKVInput{Key: key})
}

// KVGet retrieves a value from the global key-value store.
// Returns (zero, false, nil) if the key doesn't exist.
func KVGet[R any](rt *Runtime, ctx context.Context, key string) (R, bool, error) {
	encoded, err := rt.kv.GetKV(ctx, sysdb.GetKVInput{Key: key})
	if err != nil {
		return *new(R), false, err
	}
	if encoded == nil {
		return *new(R), false, nil
	}

	result, err := serialization.DecodeJSON[R](encoded)
	if err != nil {
		return *new(R), false, fmt.Errorf("turbine: failed to deserialize KV value: %w", err)
	}
	return result, true, nil
}
