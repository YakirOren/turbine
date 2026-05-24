package sysdb

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

// KV manages the pt_kv table. Global key-value store, no workflow context.
type KV struct {
	app    core.App
	logger *slog.Logger
}

func NewKV(app core.App, logger *slog.Logger) *KV {
	return &KV{
		app:    app,
		logger: logger.With("service", "kv"),
	}
}

func (k *KV) SetKV(ctx context.Context, input SetKVInput) error {
	_, err := k.app.DB().NewQuery(`INSERT INTO pt_kv (id, key, value, updated_at_epoch_ms)
		VALUES ({:id}, {:key}, {:value}, {:updated_at})
		ON CONFLICT (key)
		DO UPDATE SET value = excluded.value, updated_at_epoch_ms = excluded.updated_at_epoch_ms`).Bind(dbx.Params{
		"id":         core.GenerateDefaultRandomId(),
		"key":        input.Key,
		"value":      derefStr(input.Value),
		"updated_at": time.Now().UnixMilli(),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to set KV: %w", err)
	}
	return nil
}

func (k *KV) GetKV(ctx context.Context, input GetKVInput) (*string, error) {
	var value sql.NullString
	err := k.app.DB().Select("value").
		From("pt_kv").
		Where(dbx.HashExp{"key": input.Key}).
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

func (k *KV) DeleteKV(ctx context.Context, input DeleteKVInput) error {
	_, err := k.app.DB().Delete("pt_kv", dbx.HashExp{
		"key": input.Key,
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to delete KV: %w", err)
	}
	return nil
}
