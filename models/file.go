package models

import (
	"GoFileShare/config"
	"GoFileShare/utils"
	"database/sql"
	"fmt"
	"github.com/fatih/color"
	"log"
	"os"
)

// AddFileNode 添加文件节点到数据库
func AddFileNode(path string, name string, nodeType bool, parentID string, authLevel int) error {
	var parentIDPtr *int64
	if parentID == "" || parentID == "root" || parentID == "undefined" || parentID == "null" {
		// 根目录，使用NULL
		parentIDPtr = nil
	} else {
		// 解析父节点ID
		var pid int64
		if _, err := fmt.Sscanf(parentID, "%d", &pid); err != nil {
			return fmt.Errorf("无效的父节点ID: %s", parentID)
		}
		parentIDPtr = &pid
	}

	systemFilePath := ""
	if path != "" {
		systemFilePath = config.GetSystemFilePath(path, config.RootPath)
	}

	var typeInt int
	if nodeType {
		typeInt = 1
	} else {
		typeInt = 0
	}

	var query string
	var args []interface{}

	if parentIDPtr == nil {
		query = `INSERT INTO file_nodes (parent_id, type, name, path, effective_auth_level, system_file_path) 
				 VALUES (NULL, ?, ?, ?, ?, ?)`
		args = []interface{}{typeInt, name, path, authLevel, systemFilePath}
	} else {
		query = `INSERT INTO file_nodes (parent_id, type, name, path, effective_auth_level, system_file_path) 
				 VALUES (?, ?, ?, ?, ?, ?)`
		args = []interface{}{*parentIDPtr, typeInt, name, path, authLevel, systemFilePath}
	}

	_, err := config.FileDB.Exec(query, args...)
	return err
}

// DeleteFileNode 删除文件节点
func DeleteFileNode(nodeID int64) error {
	_, err := config.FileDB.Exec("DELETE FROM file_nodes WHERE id = ?", nodeID)
	return err
}

// DeleteFileNodeWithChildren 文件节点清除时，删除所有子节点和物理文件
func DeleteFileNodeWithChildren(nodeID string) error {
	var nid int64
	if _, err := fmt.Sscanf(nodeID, "%d", &nid); err != nil {
		return fmt.Errorf("无效的节点ID: %s", nodeID)
	}

	deque := utils.NewDeque()
	tempNodes, err := SearchFileNodeByID(nid)
	if err != nil {
		return err
	}

	if len(tempNodes) == 0 {
		return fmt.Errorf("文件节点不存在")
	}

	// 根节点加入队列
	deque.EnterQueue(tempNodes[0])

	var allNodesToDelete []config.FileNode

	// 广度优先遍历，收集所有需要删除的节点
	for deque.Len() != 0 {
		// 从队列中取出并移除元素
		currentNode := deque.RemoveQueue().(config.FileNode)
		allNodesToDelete = append(allNodesToDelete, currentNode)

		// 查找当前节点的所有子节点
		childNodes, err := SearchFileNodeByParentID(&currentNode.ID)
		if err != nil {
			return err
		}

		for _, childNode := range childNodes {
			deque.EnterQueue(childNode)
		}
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
	for _, node := range allNodesToDelete {
		if err := DeleteFileNode(node.ID); err != nil {
			return err
		}
	}

	color.Green("成功删除 %d 个文件节点记录", len(allNodesToDelete))
	return nil
}

// SearchFileNodeByID 在数据库中根据ID搜索文件节点
func SearchFileNodeByID(nodeID int64) ([]config.FileNode, error) {
	query := `SELECT id, parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path 
			  FROM file_nodes WHERE id = ?`

	rows, err := config.FileDB.Query(query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []config.FileNode
	for rows.Next() {
		var node config.FileNode
		var typeInt int
		var parentID sql.NullInt64
		var systemFilePath, netFilePath sql.NullString

		err := rows.Scan(&node.ID, &parentID, &typeInt, &node.Name, &node.Path,
			&node.EffectiveAuthLevel, &systemFilePath, &netFilePath)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}

		node.Type = typeInt == 1
		if parentID.Valid {
			node.ParentID = &parentID.Int64
		}

		if systemFilePath.Valid || netFilePath.Valid {
			node.Storage = &config.StorageLocation{
				SystemFilePath: systemFilePath.String,
				NetFilePath:    netFilePath.String,
			}
		}

		results = append(results, node)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// SearchFileNodeByParentID 在数据库中根据父节点ID搜索文件节点
func SearchFileNodeByParentID(parentID *int64) ([]config.FileNode, error) {
	var query string
	var args []interface{}

	if parentID == nil {
		// 查询根目录节点（parent_id为NULL）
		query = `SELECT id, parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path 
				 FROM file_nodes WHERE parent_id IS NULL`
		args = []interface{}{}
	} else {
		query = `SELECT id, parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path 
				 FROM file_nodes WHERE parent_id = ?`
		args = []interface{}{*parentID}
	}

	rows, err := config.FileDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []config.FileNode
	for rows.Next() {
		var node config.FileNode
		var typeInt int
		var parentIDVal sql.NullInt64
		var systemFilePath, netFilePath sql.NullString

		err := rows.Scan(&node.ID, &parentIDVal, &typeInt, &node.Name, &node.Path,
			&node.EffectiveAuthLevel, &systemFilePath, &netFilePath)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}

		node.Type = typeInt == 1
		if parentIDVal.Valid {
			node.ParentID = &parentIDVal.Int64
		}

		if systemFilePath.Valid || netFilePath.Valid {
			node.Storage = &config.StorageLocation{
				SystemFilePath: systemFilePath.String,
				NetFilePath:    netFilePath.String,
			}
		}

		results = append(results, node)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// SearchFileNodeByName 在数据库中根据名称搜索文件节点
func SearchFileNodeByName(name string) ([]config.FileNode, error) {
	query := `SELECT id, parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path 
			  FROM file_nodes WHERE name = ?`

	rows, err := config.FileDB.Query(query, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []config.FileNode
	for rows.Next() {
		var node config.FileNode
		var typeInt int
		var parentID sql.NullInt64
		var systemFilePath, netFilePath sql.NullString

		err := rows.Scan(&node.ID, &parentID, &typeInt, &node.Name, &node.Path,
			&node.EffectiveAuthLevel, &systemFilePath, &netFilePath)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}

		node.Type = typeInt == 1
		if parentID.Valid {
			node.ParentID = &parentID.Int64
		}

		if systemFilePath.Valid || netFilePath.Valid {
			node.Storage = &config.StorageLocation{
				SystemFilePath: systemFilePath.String,
				NetFilePath:    netFilePath.String,
			}
		}

		results = append(results, node)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// SearchFileNodeByNamePattern 在数据库中根据名称模式搜索文件节点（支持模糊搜索）
func SearchFileNodeByNamePattern(pattern string) ([]config.FileNode, error) {
	query := `SELECT id, parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path 
			  FROM file_nodes WHERE name LIKE ?`

	rows, err := config.FileDB.Query(query, "%"+pattern+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []config.FileNode
	for rows.Next() {
		var node config.FileNode
		var typeInt int
		var parentID sql.NullInt64
		var systemFilePath, netFilePath sql.NullString

		err := rows.Scan(&node.ID, &parentID, &typeInt, &node.Name, &node.Path,
			&node.EffectiveAuthLevel, &systemFilePath, &netFilePath)
		if err != nil {
			log.Printf("failed to scan row: %v", err)
			return nil, err
		}

		node.Type = typeInt == 1
		if parentID.Valid {
			node.ParentID = &parentID.Int64
		}

		if systemFilePath.Valid || netFilePath.Valid {
			node.Storage = &config.StorageLocation{
				SystemFilePath: systemFilePath.String,
				NetFilePath:    netFilePath.String,
			}
		}

		results = append(results, node)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// InsertFileNode 插入文件节点
func InsertFileNode(fileNode *config.FileNode) error {
	var typeInt int
	if fileNode.Type {
		typeInt = 1
	} else {
		typeInt = 0
	}

	systemFilePath := ""
	netFilePath := ""
	if fileNode.Storage != nil {
		systemFilePath = fileNode.Storage.SystemFilePath
		netFilePath = fileNode.Storage.NetFilePath
	}

	var query string
	var args []interface{}

	if fileNode.ParentID == nil {
		query = `INSERT INTO file_nodes (parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path) 
				 VALUES (NULL, ?, ?, ?, ?, ?, ?)`
		args = []interface{}{typeInt, fileNode.Name, fileNode.Path, fileNode.EffectiveAuthLevel, systemFilePath, netFilePath}
	} else {
		query = `INSERT INTO file_nodes (parent_id, type, name, path, effective_auth_level, system_file_path, net_file_path) 
				 VALUES (?, ?, ?, ?, ?, ?, ?)`
		args = []interface{}{*fileNode.ParentID, typeInt, fileNode.Name, fileNode.Path, fileNode.EffectiveAuthLevel, systemFilePath, netFilePath}
	}

	_, err := config.FileDB.Exec(query, args...)
	return err
}
