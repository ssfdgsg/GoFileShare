package models

import (
	"GoFileShare/config"
	"GoFileShare/utils"
	"context"
	"fmt"
	"github.com/fatih/color"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"os"
)

// AddFileNode 添加文件节点到数据库
func AddFileNode(path string, name string, nodeType bool, parentID string, authLevel int) error {
	var parentObjID primitive.ObjectID
	if parentID == "" || parentID == "root" || parentID == "undefined" || parentID == "null" {
		// 根目录，使用零值 ObjectID
		parentObjID = primitive.NilObjectID
	} else if primitive.IsValidObjectID(parentID) {
		// 合法 ObjectID
		var err error
		parentObjID, err = primitive.ObjectIDFromHex(parentID)
		if err != nil {
			return err
		}
	} else {
		// 非法 ID，返回错误
		return fmt.Errorf("无效的父节点ID: %s", parentID)
	}

	fileNode := &config.FileNode{
		ID:                 primitive.NewObjectID(),
		ParentID:           parentObjID,
		Name:               name,
		Type:               nodeType,
		Path:               path,
		EffectiveAuthLevel: authLevel,
		Storage: &config.StorageLocation{
			SystemFilePath: config.GetSystemFilePath(path, config.RootPath),
		},
	}

	_, err := config.FileCollection.InsertOne(context.TODO(), fileNode)
	return err
}

// DeleteFileNode 删除文件节点
func DeleteFileNode(nodeID primitive.ObjectID) error {
	_, err := config.FileCollection.DeleteMany(context.TODO(), map[string]interface{}{"_id": nodeID})
	return err
}

// DeleteFileNodeWithChildren 文件节点清除时，删除所有子节点和物理文件
func DeleteFileNodeWithChildren(nodeID string) error {
	nodeObjID, err := config.ParseObjectID(nodeID)
	if err != nil {
		return err
	}

	deque := utils.NewDeque()
	tempNodes, err := SearchFileNodeByID(nodeObjID)
	if err != nil {
		return err
	}

	if len(tempNodes) == 0 {
		return fmt.Errorf("文件节点不存在")
	}

	// ��节点加入队列
	deque.EnterQueue(tempNodes[0])

	var allNodesToDelete []config.FileNode

	// 广度优先遍历，收集所有需要删除的节点
	for deque.Len() != 0 {
		// 从队列中取出并移除元素
		currentNode := deque.RemoveQueue().(config.FileNode)
		allNodesToDelete = append(allNodesToDelete, currentNode)

		// 查找当前节点的所有子节点
		cursor, err := config.FileCollection.Find(context.TODO(), map[string]interface{}{"parent_id": currentNode.ID})
		if err != nil {
			return err
		}

		for cursor.Next(context.TODO()) {
			childNode := &config.FileNode{}
			if err := cursor.Decode(childNode); err != nil {
				cursor.Close(context.TODO())
				return err
			}
			deque.EnterQueue(*childNode)
		}
		cursor.Close(context.TODO())
	}

	// 删除所有物理文件（从叶子节点开始删除）
	for i := len(allNodesToDelete) - 1; i >= 0; i-- {
		node := allNodesToDelete[i]

		// 如果是文件（不是文件夹），删除物理文件
		if !node.Type && node.Storage != nil && node.Storage.SystemFilePath != "" {
			if err := os.Remove(node.Storage.SystemFilePath); err != nil {
				// 记录错误但继续删除数据库记录
				color.Red("删除物理文件失败: %s, 错误: %v", node.Storage.SystemFilePath, err)
			} else {
				color.Green("成功删除物理文件: %s", node.Storage.SystemFilePath)
			}
		}
	}

	// 删除数据库中的所有节点记录
	var deleteIDs []primitive.ObjectID
	for _, node := range allNodesToDelete {
		deleteIDs = append(deleteIDs, node.ID)
	}

	if len(deleteIDs) > 0 {
		_, err = config.FileCollection.DeleteMany(context.TODO(), map[string]interface{}{"_id": map[string]interface{}{"$in": deleteIDs}})
		if err != nil {
			return err
		}
		color.Green("成功删除 %d 个文件节点记录", len(deleteIDs))
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

// SearchFileNodeByParentID 在数据库中根据父节点ID搜索文件节点
func SearchFileNodeByParentID(parentID primitive.ObjectID) ([]config.FileNode, error) {
	var filter map[string]interface{}

	// 处理根目录的特殊情况
	if parentID == primitive.NilObjectID {
		// 查询parent_id为null或不存在的文���
		filter = map[string]interface{}{
			"$or": []interface{}{
				map[string]interface{}{"parent_id": nil},
				map[string]interface{}{"parent_id": primitive.NilObjectID},
				map[string]interface{}{"parent_id": map[string]interface{}{"$exists": false}},
			},
		}
	} else {
		filter = map[string]interface{}{"parent_id": parentID}
	}

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

// SearchFileNodeByNamePattern 在数��库中根据名称模式搜索文件节点（支持模糊搜索）
func SearchFileNodeByNamePattern(pattern string) ([]config.FileNode, error) {
	// 使用MongoDB的正则表达式进行模糊搜索
	filter := map[string]interface{}{
		"name": map[string]interface{}{
			"$regex":   pattern,
			"$options": "i", // 忽略大小写
		},
	}

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
