package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"../../utils"
	"github.com/fatih/color"
)

func main() {
	var (
		url      = flag.String("url", "", "下载URL")
		filename = flag.String("name", "", "文件名")
		output   = flag.String("output", "", "输出路径")
		fileID   = flag.String("id", "", "文件ID（用于恢复下载）")
		resume   = flag.Bool("resume", false, "恢复下载")
		list     = flag.Bool("list", false, "列出所有下载")
		progress = flag.Bool("progress", false, "显示进度")
		cancel   = flag.Bool("cancel", false, "取消下载")
		cli      = flag.Bool("cli", false, "启动交互式CLI")
	)
	
	flag.Parse()
	
	// 启动CLI模式
	if *cli {
		downloadCLI := utils.NewDownloadCLI()
		downloadCLI.Run()
		return
	}
	
	// 列出所有下载
	if *list {
		downloads := utils.ListAllDownloads()
		if len(downloads) == 0 {
			color.Yellow("没有找到下载记录")
			return
		}
		
		color.Cyan("下载列表:")
		for _, download := range downloads {
			progress := utils.GetDownloadProgressByID(download.FileID)
			fmt.Printf("ID: %s, 文件: %s, 状态: %s, 进度: %.2f%%\n",
				download.FileID, download.FileName, download.Status, progress)
		}
		return
	}
	
	// 显示进度
	if *progress && *fileID != "" {
		progressValue := utils.GetDownloadProgressByID(*fileID)
		fmt.Printf("下载进度: %.2f%%\n", progressValue)
		return
	}
	
	// 取消下载
	if *cancel && *fileID != "" {
		if err := utils.CancelDownloadByID(*fileID); err != nil {
			color.Red("取消下载失败: %v", err)
			os.Exit(1)
		}
		color.Green("已取消下载: %s", *fileID)
		return
	}
	
	// 恢复下载
	if *resume && *fileID != "" {
		if err := utils.ResumeDownloadByID(*fileID); err != nil {
			color.Red("恢复下载失败: %v", err)
			os.Exit(1)
		}
		return
	}
	
	// 开始新下载
	if *url != "" && *filename != "" {
		outputPath := *filename
		if *output != "" {
			outputPath = *output
			// 如果输出路径是目录，则在目录下使用文件名
			if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
				outputPath = filepath.Join(outputPath, *filename)
			}
		}
		
		// 生成文件ID
		downloadFileID := utils.GenerateID() + "_" + *filename
		
		color.Green("开始下载: %s", *filename)
		color.Cyan("URL: %s", *url)
		color.Cyan("保存路径: %s", outputPath)
		color.Cyan("文件ID: %s", downloadFileID)
		
		if err := utils.DownloadWithResume(downloadFileID, *filename, *url, outputPath); err != nil {
			color.Red("下载失败: %v", err)
			os.Exit(1)
		}
		
		color.Green("下载完成!")
		return
	}
	
	// 显示帮助
	fmt.Println("断点续传下载工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  下载文件:")
	fmt.Println("    go run main.go -url <URL> -name <文件名> [-output <输出路径>]")
	fmt.Println()
	fmt.Println("  恢复下载:")
	fmt.Println("    go run main.go -resume -id <文件ID>")
	fmt.Println()
	fmt.Println("  列出下载:")
	fmt.Println("    go run main.go -list")
	fmt.Println()
	fmt.Println("  查看进度:")
	fmt.Println("    go run main.go -progress -id <文件ID>")
	fmt.Println()
	fmt.Println("  取消下载:")
	fmt.Println("    go run main.go -cancel -id <文件ID>")
	fmt.Println()
	fmt.Println("  交互式CLI:")
	fmt.Println("    go run main.go -cli")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  go run main.go -url https://example.com/file.zip -name file.zip")
	fmt.Println("  go run main.go -resume -id abc123_file.zip")
	fmt.Println("  go run main.go -cli")
}