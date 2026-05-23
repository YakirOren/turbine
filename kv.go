package turbine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// kv manages the pt_kv table. Global key-value store, no workflow context.
type kv struct {
	app    core.App
	logger *slog.Logger
}

func newKV(app core.App, logger *slog.Logger) *kv {
	return &kv{
		app:    app,
		logger: logger.With("service", "kv"),
	}
}

func (k *kv) setKV(ctx context.Context, input setKVInput) error {
	_, err := k.app.DB().NewQuery(`INSERT INTO pt_kv (id, key, value, updated_at_epoch_ms)
		VALUES ({:id}, {:key}, {:value}, {:updated_at})
		ON CONFLICT (key)
		DO UPDATE SET value = excluded.value, updated_at_epoch_ms = excluded.updated_at_epoch_ms`).Bind(dbx.Params{
		"id":         core.GenerateDefaultRandomId(),
		"key":        input.key,
		"value":      derefStr(input.value),
		"updated_at": time.Now().UnixMilli(),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to set KV: %w", err)
	}
	return nil
}

func (k *kv) getKV(ctx context.Context, input getKVInput) (*string, error) {
	var value sql.NullString
	err := k.app.DB().Select("value").
		From("pt_kv").
		Where(dbx.HashExp{"key": input.key}).
		Row(&value)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get KV: %w", err)
	}

	if value.Valid {
		return &value.String, nil
	}
	return nil, nil
}

func (k *kv) deleteKV(ctx context.Context, input deleteKVInput) error {
	_, err := k.app.DB().Delete("pt_kv", dbx.HashExp{
		"key": input.key,
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to delete KV: %w", err)
	}
	return nil
}

// KVSet creates or overwrites a key-value pair in the global store.
func (rt *Runtime) KVSet(ctx context.Context, key string, value any) error {
	if key == "" {
		return fmt.Errorf("turbine: KV key must not be empty")
	}

	encoded, err := encodeJSON[any](value)
	if err != nil {
		return fmt.Errorf("turbine: failed to serialize KV value: %w", err)
	}
	if encoded == nil {
		return fmt.Errorf("turbine: KV value must not be nil")
	}

	return rt.kv.setKV(ctx, setKVInput{
		key:   key,
		value: encoded,
	})
}

// KVDelete removes a key from the global store. No-op if the key doesn't exist.
func (rt *Runtime) KVDelete(ctx context.Context, key string) error {
	return rt.kv.deleteKV(ctx, deleteKVInput{key: key})
}

// KVGet retrieves a value from the global key-value store.
// Returns (zero, false, nil) if the key doesn't exist.
func KVGet[R any](rt *Runtime, ctx context.Context, key string) (R, bool, error) {
	encoded, err := rt.kv.getKV(ctx, getKVInput{key: key})
	if err != nil {
		return *new(R), false, err
	}
	if encoded == nil {
		return *new(R), false, nil
	}

	result, err := decodeJSON[R](encoded)
	if err != nil {
		return *new(R), false, fmt.Errorf("turbine: failed to deserialize KV value: %w", err)
	}
	return result, true, nil
}
