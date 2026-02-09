package mysql

import (
	"context"
	"database/sql"

	"GoFileShare/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) UserExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user WHERE name = ?", username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, username, password, email string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO user(name, password, email,status) VALUES(?, ?, ?, ?)", username, password, email, 100)
	return err
}

func (r *UserRepository) ValidateUser(ctx context.Context, username, password string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user WHERE name = ? AND password = ? ", username, password).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, username string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET last_login = NOW() WHERE name = ?", username)
	return err
}

func (r *UserRepository) GetUserByName(ctx context.Context, username string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, password, email, create_time, last_login, status FROM user WHERE name = ?", username).Scan(
		&user.ID, &user.Name, &user.Password, &user.Email, &user.CreateTime, &user.LastLogin, &user.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}
