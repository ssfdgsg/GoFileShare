package models

import (
	"GoFileShare/config"
	"GoFileShare/utils"
	"context"
	"github.com/fatih/color"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

// AddFileNode 添加文件节点到数据库
func AddFileNode(path string, name string, nodeType bool, parentID string, authLevel *int) error {
	parentObjID, err := config.ParseObjectID(parentID)
	if err != nil {
		return err
	}

	fileNode := &config.FileNode{
		ID:                 primitive.NewObjectID(),
		ParentID:           parentObjID,
		Name:               name,
		Type:               nodeType,
		Path:               path,
		AuthLevel:          authLevel,
		EffectiveAuthLevel: 0,
		Storage: &config.StorageLocation{
			SystemFilePath: config.GetSystemFilePath(path, config.RootPath),
		},
	}

	_, err = config.FileCollection.InsertOne(context.TODO(), fileNode)
	return err
}

// DeleteFileNode 删除文件节点
func DeleteFileNode(nodeID primitive.ObjectID) error {
	_, err := config.FileCollection.DeleteMany(context.TODO(), map[string]interface{}{"_id": nodeID})
	return err
}

// DeleteFileNodeWithChildren 文件节点清除时，删除所有子节点
func DeleteFileNodeWithChildren(nodeID string) error {
	nodeObjID, err := config.ParseObjectID(nodeID)
	if err != nil {
		return err
	}
	deque := utils.NewDeque()
	tempNode, err := SearchFileNodeByID(nodeObjID)
	if err != nil {
		return err
	}
	deque.EnterQueue(tempNode)
	for deque.Len() != 0 {
		fileNode := deque.GetFrontElem()
		cursor, err := config.FileCollection.Find(context.TODO(), map[string]interface{}{"parent_id": fileNode.(config.FileNode).ID})
		if err != nil {
			return err
		}
		var deleteIDs []primitive.ObjectID
		for cursor.Next(context.TODO()) {
			childNode := &config.FileNode{}
			if err := cursor.Decode(childNode); err != nil {
				return err
			}
			deleteIDs = append(deleteIDs, childNode.ID)
			deque.EnterQueue(childNode)
		}
		if len(deleteIDs) > 0 {
			_, err = config.FileCollection.DeleteMany(context.TODO(), map[string]interface{}{"_id": map[string]interface{}{"$in": deleteIDs}})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SearchFileNodeByID 在数据库中根据ID搜索文件节点
func SearchFileNodeByID(nodeID primitive.ObjectID) ([]config.FileNode, error) {
	filter := map[string]interface{}{"_id": nodeID}
	cursor, err := config.FileCollection.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
			return
		}
	}(cursor, context.TODO())

	var results []config.FileNode
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}

	return results, nil
}

// SearchFileNodeByName 在数据库中根据名称搜索文件节点
func SearchFileNodeByName(name string) ([]config.FileNode, error) {
	filter := map[string]interface{}{"name": name}
	cursor, err := config.FileCollection.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			color.Red("failed to close cursor: %v", err)
			log.Fatalf("failed to close cursor: %v", err)
		}
	}(cursor, context.TODO())

	var results []config.FileNode
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}

	return results, nil
}

// InsertFileNode 插入文件节点
func InsertFileNode(fileNode *config.FileNode) error {
	_, err := config.FileCollection.InsertOne(context.TODO(), fileNode)
	return err
}
