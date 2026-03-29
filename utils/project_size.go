package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
)

type FileTypeStats struct {
	Count      int   `json:"count"`
	TotalSize  int64 `json:"total_size"`
	TotalLines int64 `json:"total_lines"`
}

type ProjectStats struct {
	TotalFiles      int                       `json:"total_files"`
	TotalDirs       int                       `json:"total_dirs"`
	TotalSize       int64                     `json:"total_size"`
	TotalLines      int64                     `json:"total_lines"`
	TotalCodeLines  int64                     `json:"total_code_lines"`
	TotalBlankLines int64                     `json:"total_blank_lines"`
	FileTypeStats   map[string]*FileTypeStats `json:"file_type_stats"`
}

type ProjectSizeCalculator struct {
	rootPath    string
	stats       *ProjectStats
	mu          sync.Mutex
	excludeDirs map[string]bool
	excludeExts map[string]bool
}

func NewProjectSizeCalculator(rootPath string) *ProjectSizeCalculator {
	return &ProjectSizeCalculator{
		rootPath: rootPath,
		stats: &ProjectStats{
			FileTypeStats: make(map[string]*FileTypeStats),
		},
		excludeDirs: map[string]bool{
			".git":         true,
			".idea":        true,
			".vscode":      true,
			"node_modules": true,
			"vendor":       true,
			".DS_Store":    true,
		},
		excludeExts: map[string]bool{
			".exe":    true,
			".dll":    true,
			".so":     true,
			".dylib":  true,
			".bin":    true,
			".dat":    true,
			".db":     true,
			".sqlite": true,
			".jpg":    true,
			".jpeg":   true,
			".png":    true,
			".gif":    true,
			".bmp":    true,
			".ico":    true,
			".svg":    true,
			".mp4":    true,
			".avi":    true,
			".mov":    true,
			".wmv":    true,
			".mp3":    true,
			".wav":    true,
			".zip":    true,
			".tar":    true,
			".gz":     true,
			".rar":    true,
			".7z":     true,
		},
	}
}

func (psc *ProjectSizeCalculator) shouldExcludeDir(dirName string) bool {
	return psc.excludeDirs[dirName]
}

func (psc *ProjectSizeCalculator) shouldExcludeFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return psc.excludeExts[ext]
}

func (psc *ProjectSizeCalculator) countLines(filePath string) (total int64, codeLines int64, blankLines int64, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		total++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			blankLines++
		} else {
			codeLines++
		}
	}

	if err := scanner.Err(); err != nil {
		return total, codeLines, blankLines, err
	}

	return total, codeLines, blankLines, nil
}

func (psc *ProjectSizeCalculator) analyzeFile(filePath string, info os.FileInfo) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		ext = "no_extension"
	}

	size := info.Size()

	psc.mu.Lock()
	defer psc.mu.Unlock()

	psc.stats.TotalFiles++
	psc.stats.TotalSize += size

	if psc.stats.FileTypeStats[ext] == nil {
		psc.stats.FileTypeStats[ext] = &FileTypeStats{}
	}
	psc.stats.FileTypeStats[ext].Count++
	psc.stats.FileTypeStats[ext].TotalSize += size

	if !psc.shouldExcludeFile(filePath) {
		total, code, blank, err := psc.countLines(filePath)
		if err == nil {
			psc.stats.TotalLines += total
			psc.stats.TotalCodeLines += code
			psc.stats.TotalBlankLines += blank
			psc.stats.FileTypeStats[ext].TotalLines += total
		}
	}

	return nil
}

func (psc *ProjectSizeCalculator) Calculate() (*ProjectStats, error) {
	err := filepath.Walk(psc.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("Error accessing path %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			if psc.shouldExcludeDir(info.Name()) && path != psc.rootPath {
				return filepath.SkipDir
			}
			psc.mu.Lock()
			psc.stats.TotalDirs++
			psc.mu.Unlock()
			return nil
		}

		return psc.analyzeFile(path, info)
	})

	if err != nil {
		logger.Error("Error walking directory %s: %v", psc.rootPath, err)
		return nil, err
	}

	return psc.stats, nil
}

func (psc *ProjectSizeCalculator) PrintStats() {
	stats := psc.stats

	color.Cyan("\n========== 项目统计信息 ==========")
	color.Green("项目路径: %s", psc.rootPath)
	color.Green("总文件数: %d", stats.TotalFiles)
	color.Green("总目录数: %d", stats.TotalDirs)
	color.Green("总大小: %s", formatBytes(stats.TotalSize))
	color.Green("总行数: %d", stats.TotalLines)
	color.Green("代码行数: %d", stats.TotalCodeLines)
	color.Green("空白行数: %d", stats.TotalBlankLines)

	color.Cyan("\n========== 文件类型统计 ==========")
	for ext, typeStats := range stats.FileTypeStats {
		if typeStats.Count > 0 {
			color.Yellow("%s: %d 文件, %s, %d 行",
				ext,
				typeStats.Count,
				formatBytes(typeStats.TotalSize),
				typeStats.TotalLines)
		}
	}
	color.Cyan("==================================\n")
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func CalculateProjectSize(rootPath string) (*ProjectStats, error) {
	calculator := NewProjectSizeCalculator(rootPath)
	stats, err := calculator.Calculate()
	if err != nil {
		return nil, err
	}
	calculator.PrintStats()
	return stats, nil
}

func GetDirectorySize(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}

func GetFileCount(dirPath string) (int, error) {
	var fileCount int

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})

	return fileCount, err
}

func CountLinesInFile(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var lineCount int64
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}

func GetLinesOfCode(dirPath string, extensions []string) (int64, error) {
	var totalLines int64
	extMap := make(map[string]bool)

	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[strings.ToLower(ext)] = true
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			dirName := info.Name()
			if dirName == ".git" || dirName == ".idea" || dirName == ".vscode" ||
				dirName == "node_modules" || dirName == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if len(extMap) == 0 || extMap[ext] {
			lines, err := CountLinesInFile(path)
			if err == nil {
				totalLines += lines
			}
		}

		return nil
	})

	return totalLines, err
}
