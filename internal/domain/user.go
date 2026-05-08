package domain

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

//go:generate go tool mockgen -source=user.go -destination=./mocks/user.go -package=mocks
type UserRepositoryInterface interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`
	ID            int       `bun:"id,pk,autoincrement"`
	Email         string    `bun:"email,notnull,unique"`
	Password      string    `bun:"password,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
