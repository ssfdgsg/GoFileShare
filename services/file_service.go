package services

import (
	"GoFileShare/utils"
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"io"
	"net/http"
	"os"
)

type DownloadTask struct {
	DownloadIO   utils.FileIOTask
	OrderId      int
	FilePath     string
	DownloadUrl  string
	RangeStart   int64
	RangeEnd     int64
	DownloadSize int64
	TaskDone     bool
	OnComplete   func(*DownloadTask)        // 完成回调
	OnError      func(*DownloadTask, error) // 错误回调
	handleError  func(*DownloadTask, error)
}

var FileTaskHttpServer = http.Client{
	Timeout: 0,
	Transport: &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DisableKeepAlives: true,
	},
}

func NewDownloadTask(url string, order int, filePath string, rangeStart int64, rangeEnd int64) *DownloadTask {
	return &DownloadTask{
		DownloadUrl:  url,
		OrderId:      order,
		FilePath:     filePath,
		RangeStart:   rangeStart,
		RangeEnd:     rangeEnd,
		DownloadSize: 0,
		TaskDone:     false,
	}
}

func (DownloadTask *DownloadTask) Execute() {
	file, e := os.OpenFile(DownloadTask.FilePath, os.O_WRONLY, 0755)
	if e != nil {
		color.Red("Error opening file for task %d: %v", DownloadTask.OrderId, e)
		color.HiRed("%s", e)
		DownloadTask.handleError(DownloadTask, e)
		return
	}

	defer func() {
		_ = file.Close()
	}()

	_, e = file.Seek(DownloadTask.RangeStart, io.SeekStart)
	if e != nil {
		color.Red("Error seeking the file for task %d: %v", DownloadTask.OrderId, e)
		color.HiRed("%s", e)
		DownloadTask.handleError(DownloadTask, e)
		return
	}

	request, e := http.NewRequest("GET", DownloadTask.DownloadUrl, nil)
	if e != nil {
		color.Red("Error creating request for download task %d: %v", DownloadTask.OrderId, e)
		color.HiRed("%s", e)
		DownloadTask.handleError(DownloadTask, e)
		return
	}

	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", DownloadTask.RangeStart, DownloadTask.RangeEnd))

	response, e := FileTaskHttpServer.Do(request)
	if e != nil {
		color.Red("Error new request to downloading file for task %d: %v", DownloadTask.OrderId, e)
		color.HiRed("%s", e)
		DownloadTask.handleError(DownloadTask, e)
		return
	}

	defer func() {
		if response != nil {
			_ = response.Body.Close()
		}
	}()

	buffer := make([]byte, 128*1024)

	writer := bufio.NewWriter(file)

	for {
		readSize, readError := response.Body.Read(buffer)
		if readError != nil && readError != io.EOF {
			// 如果读取完毕则退出循环
			color.Red("Error while reading the %d response", DownloadTask.OrderId)
			color.HiRed("%s", readError)
			DownloadTask.handleError(DownloadTask, readError)
			return
		}
		if readSize > 0 {
			_, writeError := writer.Write(buffer[:readSize])
			if writeError != nil {
				color.Red("Error while writing to file for task %d: %v", DownloadTask.OrderId, writeError)
				color.HiRed("%s", writeError)
				DownloadTask.handleError(DownloadTask, writeError)
				return
			}
			writeError = writer.Flush()
			if writeError != nil {
				color.Red("Error while flushing writer for task %d: %v", DownloadTask.OrderId, writeError)
				color.HiRed("%s", writeError)
				DownloadTask.handleError(DownloadTask, writeError)
				return
			}
		}
		DownloadTask.DownloadSize += int64(readSize)
		if readError == io.EOF {
			break
		}
	}
	DownloadTask.TaskDone = true
	DownloadTask.OnComplete(DownloadTask)
}
