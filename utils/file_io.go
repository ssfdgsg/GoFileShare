package utils

import (
	"GoFileShare/services"
	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/fatih/color"
	"golang.org/x/net/html/atom"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileIOTask struct {
	FileName      string
	FilePath      string
	FileSize      int64
	DownloadUrl   string
	OffSet        int64
	ReadAtOffSet  func()
	WriteAtOffSet func()
}

func ReadAtOffset(fileName string, offset int64, size int) ([]byte, error) {
	file, err := os.Open(fileName)
	if err != nil {
		color.Red("Error opening file %s: %v", fileName, err)
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			color.Red("Error closing file %s: %v", fileName, err)
		} else {
			color.Green("File %s closed successfully.", fileName)
		}
	}(file)

	data := make([]byte, size)
	_, err = file.ReadAt(data, offset)
	if err != nil {
		color.Red("Error reading at offset %d from file %s: %v", offset, fileName, err)
		return nil, err
	}

	return data, nil
}

func WriteAtOffset(fileName string, offset int64, data []byte) error {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		color.Red("Error opening file %s: %v", fileName, err)
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			color.Red("Error closing file %s: %v", fileName, err)
		} else {
			color.Green("File %s closed successfully.", fileName)
		}
	}(file)

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		color.Red("Error seeking to offset %d from file %s: %v", offset, fileName, err)
		return err
	}
	_, err = file.WriteAt(data, offset)
	if err != nil {
		color.Red("Error writing to file %s: %v", fileName, err)
	}
	return err
}

func MD5Check(fileName string) string {
	file, err := os.Open(fileName)
	if err != nil {
		color.Red("Error opening file %s: %v", fileName, err)
		return "READ_FILE_ERROR"
	}
	hasher := md5.New()
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				color.Red("Error reading file %s: %v", fileName, err)
			}
			break
		}
		hasher.Write(buf[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

func createZipFile(zipPath string) (*zip.Writer, *os.File, error) {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return nil, nil, err
	}
	zipWriter := zip.NewWriter(zipFile)
	return zipWriter, zipFile, nil
}

func addFilesToZip(zipWriter *zip.Writer, basePath, rootPath string) error {
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		zipFile, err := zipWriter.Create(relativePath)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				color.Red("Error closing file %s: %v", path, err)
			} else {
				color.Green("File %s closed successfully.", path)
			}
		}(file)
		_, err = io.Copy(zipFile, file)
		return err
	})
	return err
}

// 搜索指定目录下的所有文件，并将其压缩到指定的zip文件中
func compressFolder(sourcePath, zipPath string) error {
	zipWriter, zipFile, err := createZipFile(zipPath)
	if err != nil {
		return err
	}
	defer func(zipFile *os.File) {
		err := zipFile.Close()
		if err != nil {
			color.Red("Error closing zip file: %v", err)
		} else {
			color.Green("Zip file created successfully: %s", zipPath)
		}
	}(zipFile)
	defer func(zipWriter *zip.Writer) {
		err := zipWriter.Close()
		if err != nil {
			color.Red("Error closing zip writer: %v", err)
		} else {
			color.Green("Zip writer closed successfully.")
		}
	}(zipWriter)
	err = addFilesToZip(zipWriter, sourcePath, filepath.Dir(sourcePath))
	return err
}

func UnzipTask(zipPath, destPath string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		color.Red("Error opening zip file %s: %v", zipPath, err)
		return err
	}
	defer func() {
		if err := zipReader.Close(); err != nil {
			color.Red("Error closing zip reader: %v", err)
		} else {
			color.Green("Zip reader closed successfully.")
		}
	}()

	for _, f := range zipReader.File {
		path := filepath.Join(destPath, f.Name)
		if f.FileInfo().IsDir() {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			err = outFile.Close()
			if err != nil {
				color.Red("Error closing output file %s: %v", path, err)
				return err
			}
			return err
		}
		_, err = io.Copy(outFile, rc)
		err = outFile.Close()
		if err != nil {
			color.Red("Error closing output file %s: %v", path, err)
			return err
		}
		err = rc.Close()
		if err != nil {
			color.Red("Error closing zip file reader for %s: %v", f.Name, err)
			return err
		}
	}
	return nil
}

// GetZipFileCount 返回 ZIP 文件中非目录条目的数量
func GetZipFileCount(zipFilePath string) (int, error) {
	r, err := zip.OpenReader(zipFilePath)
	if err != nil {
		color.Red("Error opening zip file %s: %v", zipFilePath, err)
		return 0, err
	}
	defer func() {
		// 在这里，err 是 r.Close() 的返回值，而不是 GetZipFileCount 外部的 err
		if closeErr := r.Close(); closeErr != nil {
			color.Red("Error closing zip reader: %v", closeErr)
		} else {
			color.Green("Zip reader closed successfully.")
		}
	}()
	fileCount := 0
	for _, f := range r.File {
		// 检查条目是否是目录。如果是目录，则跳过不计数。
		if !f.FileInfo().IsDir() {
			fileCount++
		}
	}
	return fileCount, nil
}

func GenerateID() string {
	// 生成一个简单的唯一ID，可以使用时间戳和随机数
	return hex.EncodeToString(md5.New().Sum([]byte(filepath.Base(os.TempDir()))))[:16]
}

func GetStopWork() {
	file, err := os.OpenFile("./data/stop_work.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		color.Red("Error opening stop_work.txt: %v", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			color.Red("Error closing stop_work.txt: %v", err)
		}
	}(file)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines := strings.Split(line, " ")
		tempUrl := lines[0]
		tempFilePath := lines[1]
		tempStatus := lines[2]
		Async(services.StartDownload(  tempStatus), func(task *services.DownloadTask) {)
		}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取文件出错:", err)
	}

}
