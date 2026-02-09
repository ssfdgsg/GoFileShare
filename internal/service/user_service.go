package service

import (
	"context"

	"GoFileShare/internal/domain"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) UserExists(ctx context.Context, username string) (bool, error) {
	return s.repo.UserExists(ctx, username)
}

func (s *UserService) CreateUser(ctx context.Context, username, password, email string) error {
	return s.repo.CreateUser(ctx, username, password, email)
}

func (s *UserService) ValidateUser(ctx context.Context, username, password string) (bool, error) {
	return s.repo.ValidateUser(ctx, username, password)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, username string) error {
	return s.repo.UpdateLastLogin(ctx, username)
}

func (s *UserService) GetUserByName(ctx context.Context, username string) (*domain.User, error) {
	return s.repo.GetUserByName(ctx, username)
}
