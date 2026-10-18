package main

import (
	"GoFileShare/utils"
	"fmt"
	"os"
)

func main() {
	args := os.Args
	var targetPath string

	if len(args) > 1 {
		targetPath = args[1]
	} else {
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("无法获取当前目录: %v\n", err)
			os.Exit(1)
		}
		targetPath = currentDir
	}

	stats, err := utils.CalculateProjectSize(targetPath)
	if err != nil {
		fmt.Printf("计算项目大小时出错: %v\n", err)
		os.Exit(1)
	}

	_ = stats
}
