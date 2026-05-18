package domain

import (
	"context"
)

//go:generate go tool mockgen -source=user.go -destination=./mocks/user.go -package=mocks
type UserRepositoryInterface interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type User struct {
	ID       int    `gorm:"column:id;primaryKey;type:bigint;not null;autoIncrement"`
	Email    string `gorm:"column:email;type:varchar(255);not null;unique"`
	Password string `gorm:"column:password;type:varchar(100);not null"`
	Timestamp
}
