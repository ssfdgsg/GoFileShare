package config

// AuthCheck 权限检查（当前系统默认允许全部访问）
func AuthCheck(AuthLevel int, FileNodes []FileNode) ([]FileNode, error) {
	return FileNodes, nil
}
