package models

import "GoFileShare/config"

type File struct {
	ID       int    `json:"id" db:"id"`
	FileName string `json:"file_name" db:"file_name"`
	FileSize int64  `json:"file_size" db:"file_size"`
	FilePath string `json:"file_path" db:"file_path"`
}

func CreateFileRecord(file *File) error {
	query := `
		INSERT INTO files (file_name,file_size,file_path)
		VALUES (?, ?, ?, ?)
	`

	_, err := config.DB.Exec(query,
		file.FileName,
		file.FileSize, file.FilePath,
	)

	return err
}
