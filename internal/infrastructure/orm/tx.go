package orm

import (
	"context"

	"gorm.io/gorm"
)

type Tx struct {
	*DB
}

func NewTx(db *DB) *Tx {
	return &Tx{db}
}

func (t *Tx) Do(ctx context.Context, fc func(ctx context.Context) error) error {
	return t.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = context.WithValue(ctx, txCtxKey, tx)
		return fc(ctx)
	})
}
