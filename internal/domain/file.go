package domain

import "go.mongodb.org/mongo-driver/bson/primitive"

// StorageLocation represents a concrete storage location for a file.
type StorageLocation struct {
	SystemFilePath string `bson:"system_file_path"` // Physical storage path on disk.
	NetFilePath    string `bson:"net_file_path"`    // Optional network path when system path exists.
}

// FileNode represents a logical file or directory node.
type FileNode struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	ParentID primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id"`
	Type     bool               `bson:"type" json:"type"` // false=file, true=directory
	Name     string             `bson:"name" json:"name"`
	Path     string             `bson:"path" json:"path"`
	// EffectiveAuthLevel is the resolved auth level for this node.
	EffectiveAuthLevel int              `bson:"effective_auth_level" json:"auth_level"`
	Storage            *StorageLocation `bson:"storage,omitempty" json:"storage,omitempty"`
}
