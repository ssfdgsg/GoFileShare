package domain

import "context"

type User struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Password   string     `json:"password"`
	Email      string     `json:"email"`
	CreateTime string     `json:"create_time"`
	LastLogin  *string    `json:"last_login"`
	Status     int        `json:"status"`
}

type UserRepository interface {
	UserExists(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, username, password, email string) error
	ValidateUser(ctx context.Context, username, password string) (bool, error)
	UpdateLastLogin(ctx context.Context, username string) error
	GetUserByName(ctx context.Context, username string) (*User, error)
}

type FileRepository interface {
	AddFileNode(ctx context.Context, path, name string, nodeType bool, parentID string, authLevel int) error
	DeleteFileNode(ctx context.Context, nodeID string) error
	DeleteFileNodeWithChildren(ctx context.Context, nodeID string) error
	SearchFileNodeByID(ctx context.Context, nodeID string) ([]FileNode, error)
	SearchFileNodeByParentID(ctx context.Context, parentID string) ([]FileNode, error)
	SearchFileNodeByName(ctx context.Context, name string) ([]FileNode, error)
	SearchFileNodeByNamePattern(ctx context.Context, pattern string) ([]FileNode, error)
	InsertFileNode(ctx context.Context, node *FileNode) error
}
