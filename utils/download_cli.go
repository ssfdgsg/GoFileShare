package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

// DownloadCLI 下载命令行工具
type DownloadCLI struct {
	downloader *ResumableDownloader
}

// NewDownloadCLI 创建下载CLI
func NewDownloadCLI() *DownloadCLI {
	// 使用用户主目录下的.downloads作为状态目录
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	return &DownloadCLI{
		downloader: NewResumableDownloader(stateDir),
	}
}

// Run 运行CLI
func (cli *DownloadCLI) Run() {
	color.Cyan("=== 断点续传下载工具 ===")
	color.Green("输入 'help' 查看可用命令")
	
	// 启动时自动恢复未完成的下载
	cli.downloader.ResumeAllIncompleteDownloads()
	
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		color.Yellow("\n> ")
		fmt.Print("> ")
		
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		parts := strings.Fields(input)
		command := parts[0]
		
		switch command {
		case "help":
			cli.showHelp()
		case "download", "dl":
			cli.handleDownload(parts[1:])
		case "resume":
			cli.handleResume(parts[1:])
		case "pause":
			cli.handlePause(parts[1:])
		case "cancel":
			cli.handleCancel(parts[1:])
		case "list", "ls":
			cli.handleList()
		case "progress", "prog":
			cli.handleProgress(parts[1:])
		case "resume-all":
			cli.handleResumeAll()
		case "quit", "exit", "q":
			color.Green("再见!")
			return
		default:
			color.Red("未知命令: %s. 输入 'help' 查看可用命令", command)
		}
	}
}

// showHelp 显示帮助信息
func (cli *DownloadCLI) showHelp() {
	color.Cyan("\n可用命令:")
	fmt.Println("  download <URL> <文件名> [保存路径] - 开始新的下载")
	fmt.Println("  resume <文件ID> - 恢复指定下载")
	fmt.Println("  pause <文件ID> - 暂停指定下载")
	fmt.Println("  cancel <文件ID> - 取消指定下载")
	fmt.Println("  list - 列出所有下载")
	fmt.Println("  progress <文件ID> - 查看下载进度")
	fmt.Println("  resume-all - 恢复所有未完成的下载")
	fmt.Println("  help - 显示此帮助信息")
	fmt.Println("  quit/exit/q - 退出程序")
}

// handleDownload 处理下载命令
func (cli *DownloadCLI) handleDownload(args []string) {
	if len(args) < 2 {
		color.Red("用法: download <URL> <文件名> [保存路径]")
		return
	}
	
	url := args[0]
	fileName := args[1]
	
	// 默认保存路径
	savePath := fileName
	if len(args) > 2 {
		savePath = args[2]
	}
	
	// 如果保存路径是目录，则在目录下使用文件名
	if info, err := os.Stat(savePath); err == nil && info.IsDir() {
		savePath = filepath.Join(savePath, fileName)
	}
	
	// 生成文件ID
	fileID := GenerateID() + "_" + fileName
	
	// 默认分块大小为1MB
	chunkSize := int64(1024 * 1024)
	
	color.Green("开始下载: %s", fileName)
	color.Cyan("URL: %s", url)
	color.Cyan("保存路径: %s", savePath)
	color.Cyan("文件ID: %s", fileID)
	
	if err := cli.downloader.StartDownload(fileID, fileName, url, savePath, chunkSize); err != nil {
		color.Red("下载失败: %v", err)
	}
}

// handleResume 处理恢复命令
func (cli *DownloadCLI) handleResume(args []string) {
	if len(args) < 1 {
		color.Red("用法: resume <文件ID>")
		return
	}
	
	fileID := args[0]
	
	if err := cli.downloader.ResumeDownload(fileID); err != nil {
		color.Red("恢复下载失败: %v", err)
	}
}

// handlePause 处理暂停命令
func (cli *DownloadCLI) handlePause(args []string) {
	if len(args) < 1 {
		color.Red("用法: pause <文件ID>")
		return
	}
	
	fileID := args[0]
	
	if err := cli.downloader.PauseDownload(fileID); err != nil {
		color.Red("暂停下载失败: %v", err)
	} else {
		color.Green("已暂停下载: %s", fileID)
	}
}

// handleCancel 处理取消命令
func (cli *DownloadCLI) handleCancel(args []string) {
	if len(args) < 1 {
		color.Red("用法: cancel <文件ID>")
		return
	}
	
	fileID := args[0]
	
	if err := cli.downloader.CancelDownload(fileID); err != nil {
		color.Red("取消下载失败: %v", err)
	} else {
		color.Green("已取消下载: %s", fileID)
	}
}

// handleList 处理列表命令
func (cli *DownloadCLI) handleList() {
	downloads := cli.downloader.ListDownloads()
	
	if len(downloads) == 0 {
		color.Yellow("没有找到下载记录")
		return
	}
	
	color.Cyan("\n下载列表:")
	color.Cyan("%-20s %-30s %-15s %-10s %-20s", "文件ID", "文件名", "状态", "进度", "更新时间")
	color.Cyan(strings.Repeat("-", 100))
	
	for _, download := range downloads {
		progress := cli.downloader.GetDownloadProgress(download.FileID)
		
		// 截断长文件名
		displayName := download.FileName
		if len(displayName) > 28 {
			displayName = displayName[:25] + "..."
		}
		
		// 截断长文件ID
		displayID := download.FileID
		if len(displayID) > 18 {
			displayID = displayID[:15] + "..."
		}
		
		statusColor := color.WhiteString
		switch download.Status {
		case "completed":
			statusColor = color.GreenString
		case "downloading":
			statusColor = color.CyanString
		case "paused":
			statusColor = color.YellowString
		case "failed":
			statusColor = color.RedString
		}
		
		fmt.Printf("%-20s %-30s %-15s %8.2f%% %-20s\n",
			displayID,
			displayName,
			statusColor(download.Status),
			progress,
			download.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
}

// handleProgress 处理进度命令
func (cli *DownloadCLI) handleProgress(args []string) {
	if len(args) < 1 {
		color.Red("用法: progress <文件ID>")
		return
	}
	
	fileID := args[0]
	
	state, exists := cli.downloader.stateManager.GetDownloadState(fileID)
	if !exists {
		color.Red("未找到文件ID: %s", fileID)
		return
	}
	
	progress := cli.downloader.GetDownloadProgress(fileID)
	
	color.Cyan("\n下载详情:")
	fmt.Printf("文件ID: %s\n", state.FileID)
	fmt.Printf("文件名: %s\n", state.FileName)
	fmt.Printf("文件大小: %.2f MB\n", float64(state.FileSize)/(1024*1024))
	fmt.Printf("已下载: %.2f MB\n", float64(state.Downloaded)/(1024*1024))
	fmt.Printf("进度: %.2f%%\n", progress)
	fmt.Printf("状态: %s\n", state.Status)
	fmt.Printf("分块数: %d\n", len(state.Chunks))
	
	// 显示分块状态
	completedChunks := 0
	for _, chunk := range state.Chunks {
		if chunk.Downloaded {
			completedChunks++
		}
	}
	fmt.Printf("已完成分块: %d/%d\n", completedChunks, len(state.Chunks))
	fmt.Printf("创建时间: %s\n", state.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("更新时间: %s\n", state.UpdatedAt.Format("2006-01-02 15:04:05"))
}

// handleResumeAll 处理恢复所有命令
func (cli *DownloadCLI) handleResumeAll() {
	if err := cli.downloader.ResumeAllIncompleteDownloads(); err != nil {
		color.Red("恢复所有下载失败: %v", err)
	}
}

// StartProgressMonitor 启动进度监控（可选功能）
func (cli *DownloadCLI) StartProgressMonitor() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			downloads := cli.downloader.ListDownloads()
			activeDownloads := 0
			
			for _, download := range downloads {
				if download.Status == "downloading" {
					activeDownloads++
					progress := cli.downloader.GetDownloadProgress(download.FileID)
					color.Cyan("[监控] %s: %.2f%%", download.FileName, progress)
				}
			}
			
			if activeDownloads == 0 {
				// 没有活跃下载时停止监控
				return
			}
		}
	}()
}