package repository

import (
	"context"
	"errors"

	"gss/internal/domain"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"

	"gorm.io/gorm"
)

type userRepository struct {
	db     *orm.DB
	logger *logger.Logger
}

var _ domain.UserRepositoryInterface = (*userRepository)(nil)

func NewUserRepository(db *orm.DB, logger *logger.Logger) *userRepository {
	return &userRepository{db: db, logger: logger}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	cdb := r.db.WithContext(ctx)

	err := cdb.Create(user).Error
	if err != nil {
		return errcode.WithCause(errcode.Internal, err)
	}

	return nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	cdb := r.db.WithContext(ctx)

	var user domain.User
	err := cdb.Where("email = ?", email).Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.WithCause(errcode.NotFound, err)
		}

		return nil, errcode.WithCause(errcode.Internal, err)
	}

	return &user, nil
}
