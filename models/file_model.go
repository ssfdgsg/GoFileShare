package models

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// FileNode 文件节点模型
type FileNode struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	ParentID  string             `bson:"parent_id" json:"parent_id"`
	IsDir     bool               `bson:"is_dir" json:"is_dir"`
	AuthLevel int                `bson:"auth_level" json:"auth_level"`
	Storage   FileStorage        `bson:"storage" json:"storage"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// FileStorage 文件存储信息
type FileStorage struct {
	SystemFilePath string `bson:"system_file_path" json:"system_file_path"`
	Size           int64  `bson:"size" json:"size"`
}

var fileCollection *mongo.Collection
var rootPath string

// SetFileCollection 设置文件集合
func SetFileCollection(collection *mongo.Collection, root string) {
	fileCollection = collection
	rootPath = root
}

// AddFileNode 添加文件节点
func AddFileNode(ctx context.Context, path, name string, isDir bool, parentID string, authLevel int) error {
	fileNode := &FileNode{
		ID:        primitive.NewObjectID(),
		Name:      name,
		ParentID:  parentID,
		IsDir:     isDir,
		AuthLevel: authLevel,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if !isDir && path != "" {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		fileNode.Storage = FileStorage{
			SystemFilePath: path,
			Size:           fileInfo.Size(),
		}
	}

	_, err := fileCollection.InsertOne(ctx, fileNode)
	return err
}

// DeleteFileNode 删除文件节点
func DeleteFileNode(ctx context.Context, nodeID string) error {
	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		return err
	}
	_, err = fileCollection.DeleteOne(ctx, bson.M{"_id": objID})
	return err
}

// DeleteFileNodeWithChildren 删除文件节点及其所有子节点
func DeleteFileNodeWithChildren(ctx context.Context, nodeID string) error {
	nodes, err := SearchFileNodeByID(ctx, nodeID)
	if err != nil || len(nodes) == 0 {
		return err
	}

	node := nodes[0]
	
	// 如果是目录，递归删除子节点
	if node.IsDir {
		children, err := SearchFileNodeByParentID(ctx, nodeID)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := DeleteFileNodeWithChildren(ctx, child.ID.Hex()); err != nil {
				return err
			}
		}
	} else {
		// 删除物理文件
		if node.Storage.SystemFilePath != "" {
			os.Remove(node.Storage.SystemFilePath)
		}
	}

	return DeleteFileNode(ctx, nodeID)
}

// SearchFileNodeByID 根据ID搜索文件节点
func SearchFileNodeByID(ctx context.Context, nodeID string) ([]FileNode, error) {
	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		return nil, err
	}

	cursor, err := fileCollection.Find(ctx, bson.M{"_id": objID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []FileNode
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// SearchFileNodeByParentID 根据父节点ID搜索文件节点
func SearchFileNodeByParentID(ctx context.Context, parentID string) ([]FileNode, error) {
	cursor, err := fileCollection.Find(ctx, bson.M{"parent_id": parentID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []FileNode
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// SearchFileNodeByName 根据名称搜索文件节点
func SearchFileNodeByName(ctx context.Context, name string) ([]FileNode, error) {
	cursor, err := fileCollection.Find(ctx, bson.M{"name": name})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []FileNode
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// SearchFileNodeByNamePattern 根据名称模式搜索文件节点（模糊搜索）
func SearchFileNodeByNamePattern(ctx context.Context, pattern string) ([]FileNode, error) {
	filter := bson.M{"name": bson.M{"$regex": pattern, "$options": "i"}}
	cursor, err := fileCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []FileNode
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// InsertFileNode 插入文件节点
func InsertFileNode(ctx context.Context, fileNode *FileNode) error {
	_, err := fileCollection.InsertOne(ctx, fileNode)
	return err
}

// GetAuthLevel 获取权限级别（实现config.FileNode接口）
func (f FileNode) GetAuthLevel() int {
	return f.AuthLevel
}
