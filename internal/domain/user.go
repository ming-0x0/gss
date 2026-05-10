package domain

import (
	"context"

	"github.com/uptrace/bun"
)

//go:generate go tool mockgen -source=user.go -destination=./mocks/user.go -package=mocks
type UserRepositoryInterface interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`
	ID            int    `bun:"id,pk,autoincrement"`
	Email         string `bun:"email,notnull,unique"`
	Password      string `bun:"password,notnull"`
	TimestampWithDeletedAt
}
