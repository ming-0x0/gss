package repository

import (
	"gss/internal/domain"
	"gss/internal/infrastructure/logger"
	"gss/internal/infrastructure/orm"
)

type RepositoryContainer struct {
	UserRepository domain.UserRepositoryInterface
}

func NewRepositoryContainer(
	db *orm.DB,
	logger *logger.Logger,
) *RepositoryContainer {
	return &RepositoryContainer{
		UserRepository: NewUserRepository(db, logger),
	}
}
