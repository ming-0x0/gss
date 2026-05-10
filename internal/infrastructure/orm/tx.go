package orm

import (
	"context"

	ctxkey "gss/pkg/ctxutil/key"

	"github.com/uptrace/bun"
)

type Tx struct {
	db *bun.DB
}

func NewTx(db *bun.DB) *Tx {
	return &Tx{db: db}
}

func (t *Tx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		ctx = context.WithValue(ctx, ctxkey.TransactionContextKey, tx)
		return fn(ctx)
	})
}
