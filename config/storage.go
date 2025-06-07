package config

const (
	TempDir     = "../tempFile"
	MaxFileSize = 100 << 20
	ChunkSize   = 1 << 20
)

func GetTempPath(filename string) string {
	return TempDir + "/" + filename
}
