package service

import (
	"context"
	"fmt"

	"GoFileShare/internal/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FileService struct {
	repo domain.FileRepository
}

func NewFileService(repo domain.FileRepository) *FileService {
	return &FileService{repo: repo}
}

func (s *FileService) AddFileNode(ctx context.Context, path, name string, nodeType bool, parentID string, authLevel int) error {
	return s.repo.AddFileNode(ctx, path, name, nodeType, parentID, authLevel)
}

func (s *FileService) DeleteFileNode(ctx context.Context, nodeID string) error {
	return s.repo.DeleteFileNode(ctx, nodeID)
}

func (s *FileService) DeleteFileNodeWithChildren(ctx context.Context, nodeID string) error {
	return s.repo.DeleteFileNodeWithChildren(ctx, nodeID)
}

func (s *FileService) SearchFileNodeByID(ctx context.Context, nodeID string) ([]domain.FileNode, error) {
	return s.repo.SearchFileNodeByID(ctx, nodeID)
}

func (s *FileService) SearchFileNodeByParentID(ctx context.Context, parentID string) ([]domain.FileNode, error) {
	return s.repo.SearchFileNodeByParentID(ctx, parentID)
}

func (s *FileService) SearchFileNodeByName(ctx context.Context, name string) ([]domain.FileNode, error) {
	return s.repo.SearchFileNodeByName(ctx, name)
}

func (s *FileService) SearchFileNodeByNamePattern(ctx context.Context, pattern string) ([]domain.FileNode, error) {
	return s.repo.SearchFileNodeByNamePattern(ctx, pattern)
}

func (s *FileService) InsertFileNode(ctx context.Context, node *domain.FileNode) error {
	return s.repo.InsertFileNode(ctx, node)
}

func (s *FileService) ParseObjectID(id string) (primitive.ObjectID, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return objID, nil
}

func (s *FileService) ValidateObjectID(id string) error {
	if !primitive.IsValidObjectID(id) {
		return fmt.Errorf("无效的文件节点ID")
	}
	return nil
}
