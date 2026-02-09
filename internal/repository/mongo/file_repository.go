package mongo

import (
	"context"
	"fmt"
	"log"
	"os"

	"GoFileShare/internal/domain"
	"GoFileShare/utils"

	"github.com/fatih/color"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileRepository struct {
	collection *mongo.Collection
	rootPath   string
}

func NewFileRepository(collection *mongo.Collection, rootPath string) *FileRepository {
	return &FileRepository{collection: collection, rootPath: rootPath}
}

func (r *FileRepository) AddFileNode(ctx context.Context, path, name string, nodeType bool, parentID string, authLevel int) error {
	var parentObjID primitive.ObjectID
	if parentID == "" || parentID == "root" || parentID == "undefined" || parentID == "null" {
		parentObjID = primitive.NilObjectID
	} else if primitive.IsValidObjectID(parentID) {
		var err error
		parentObjID, err = primitive.ObjectIDFromHex(parentID)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("无效的父节点ID: %s", parentID)
	}

	fileNode := &domain.FileNode{
		ID:                 primitive.NewObjectID(),
		ParentID:           parentObjID,
		Name:               name,
		Type:               nodeType,
		Path:               path,
		EffectiveAuthLevel: authLevel,
		Storage: &domain.StorageLocation{
			SystemFilePath: getSystemFilePath(path, r.rootPath),
		},
	}

	_, err := r.collection.InsertOne(ctx, fileNode)
	return err
}

func (r *FileRepository) DeleteFileNode(ctx context.Context, nodeID string) error {
	objectID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		return err
	}
	_, err = r.collection.DeleteMany(ctx, map[string]interface{}{"_id": objectID})
	return err
}

func (r *FileRepository) DeleteFileNodeWithChildren(ctx context.Context, nodeID string) error {
	nodeObjID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		return err
	}

	deque := utils.NewDeque()
	tempNodes, err := r.searchFileNodeByID(ctx, nodeObjID)
	if err != nil {
		return err
	}

	if len(tempNodes) == 0 {
		return fmt.Errorf("文件节点不存在")
	}

	deque.EnterQueue(tempNodes[0])

	var allNodesToDelete []domain.FileNode

	for deque.Len() != 0 {
		currentNode := deque.RemoveQueue().(domain.FileNode)
		allNodesToDelete = append(allNodesToDelete, currentNode)

		cursor, err := r.collection.Find(ctx, map[string]interface{}{"parent_id": currentNode.ID})
		if err != nil {
			return err
		}

		for cursor.Next(ctx) {
			childNode := &domain.FileNode{}
			if err := cursor.Decode(childNode); err != nil {
				cursor.Close(ctx)
				return err
			}
			deque.EnterQueue(*childNode)
		}
		cursor.Close(ctx)
	}

	for i := len(allNodesToDelete) - 1; i >= 0; i-- {
		node := allNodesToDelete[i]
		if !node.Type && node.Storage != nil && node.Storage.SystemFilePath != "" {
			if err := os.Remove(node.Storage.SystemFilePath); err != nil {
				color.Red("删除物理文件失败: %s, 错误: %v", node.Storage.SystemFilePath, err)
			} else {
				color.Green("成功删除物理文件: %s", node.Storage.SystemFilePath)
			}
		}
	}

	var deleteIDs []primitive.ObjectID
	for _, node := range allNodesToDelete {
		deleteIDs = append(deleteIDs, node.ID)
	}

	if len(deleteIDs) > 0 {
		_, err = r.collection.DeleteMany(ctx, map[string]interface{}{"_id": map[string]interface{}{"$in": deleteIDs}})
		if err != nil {
			return err
		}
		color.Green("成功删除 %d 个文件节点记录", len(deleteIDs))
	}

	return nil
}

func (r *FileRepository) SearchFileNodeByID(ctx context.Context, nodeID string) ([]domain.FileNode, error) {
	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		return nil, err
	}
	return r.searchFileNodeByID(ctx, objID)
}

func (r *FileRepository) SearchFileNodeByParentID(ctx context.Context, parentID string) ([]domain.FileNode, error) {
	var filter map[string]interface{}
	if parentID == "" || parentID == "root" {
		filter = map[string]interface{}{
			"$or": []interface{}{
				map[string]interface{}{"parent_id": nil},
				map[string]interface{}{"parent_id": primitive.NilObjectID},
				map[string]interface{}{"parent_id": map[string]interface{}{"$exists": false}},
			},
		}
	} else {
		objID, err := primitive.ObjectIDFromHex(parentID)
		if err != nil {
			return nil, err
		}
		filter = map[string]interface{}{"parent_id": objID}
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var results []domain.FileNode
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *FileRepository) SearchFileNodeByName(ctx context.Context, name string) ([]domain.FileNode, error) {
	filter := map[string]interface{}{"name": name}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var results []domain.FileNode
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *FileRepository) SearchFileNodeByNamePattern(ctx context.Context, pattern string) ([]domain.FileNode, error) {
	filter := map[string]interface{}{
		"name": map[string]interface{}{
			"$regex":   pattern,
			"$options": "i",
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var results []domain.FileNode
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *FileRepository) InsertFileNode(ctx context.Context, node *domain.FileNode) error {
	_, err := r.collection.InsertOne(ctx, node)
	return err
}

func (r *FileRepository) searchFileNodeByID(ctx context.Context, nodeID primitive.ObjectID) ([]domain.FileNode, error) {
	filter := map[string]interface{}{"_id": nodeID}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var results []domain.FileNode
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func getSystemFilePath(path string, rootPath string) string {
	systemPath := path + rootPath
	_, err := os.Stat(systemPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(systemPath, 0755); err != nil {
				return ""
			}
		} else {
			return ""
		}
	}
	return systemPath
}
