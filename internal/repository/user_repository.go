package repository

import (
	"context"
	"database/sql"
	"errors"

	"gss/internal/domain"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm/bunutil"

	"github.com/uptrace/bun"
)

type userRepository struct {
	db     *bunutil.DB
	logger *logger.Logger
}

var _ domain.UserRepositoryInterface = (*userRepository)(nil)

func NewUserRepository(db *bunutil.DB, logger *logger.Logger) *userRepository {
	return &userRepository{db: db, logger: logger}
}

func (u *userRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := u.db.WithContext(ctx).NewInsert().Model(user).Exec(ctx)
	if err != nil {
		return errcode.WithCause(errcode.Internal, err)
	}

	return nil
}

func (u *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user *domain.User
	err := u.db.WithContext(ctx).NewSelect().Model(user).Where("email = ?", bun.Ident(email)).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.WithCause(errcode.NotFound, err)
		}

		return nil, errcode.WithCause(errcode.Internal, err)
	}

	return user, nil
}
