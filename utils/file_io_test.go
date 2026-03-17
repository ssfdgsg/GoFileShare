package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"runtime"
	"testing"
	"time"
)

// 创建测试文件
func createTestFile(t *testing.T, size int64) string {
	tmpFile, err := os.CreateTemp("", "test_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	// 写入随机数据
	data := make([]byte, 1024*1024) // 1MB buffer
	for i := range data {
		data[i] = byte(i % 256)
	}

	written := int64(0)
	for written < size {
		toWrite := size - written
		if toWrite > int64(len(data)) {
			toWrite = int64(len(data))
		}
		n, err := tmpFile.Write(data[:toWrite])
		if err != nil {
			t.Fatal(err)
		}
		written += int64(n)
	}

	return tmpFile.Name()
}

// 直接 MD5 计算（不使用 WorkPool，一次性读取）
func calculateMD5Direct(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}



// 测试单线程 MD5
func TestMD5Check(t *testing.T) {
	// 创建 100MB 测试文件
	testFile := createTestFile(t, 100*1024*1024)
	defer os.Remove(testFile)

	start := time.Now()
	hash := MD5Check(testFile)
	duration := time.Since(start)

	t.Logf("单线程 MD5: %s", hash)
	t.Logf("耗时: %v", duration)

	if hash == "" || hash == "READ_FILE_ERROR" {
		t.Fatal("MD5 计算失败")
	}
}

// 测试并发 MD5
func TestMD5CheckConcurrent(t *testing.T) {
	// 创建 100MB 测试文件
	testFile := createTestFile(t, 100*1024*1024)
	defer os.Remove(testFile)

	start := time.Now()
	hash, err := MD5CheckConcurrent(testFile, 4)
	duration := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("并发 MD5 (4 workers): %s", hash)
	t.Logf("耗时: %v", duration)

	if hash == "" {
		t.Fatal("MD5 计算失败")
	}
}

// 性能对比测试
func TestMD5Performance(t *testing.T) {
	sizes := []int64{
		10 * 1024 * 1024,   // 10MB
		50 * 1024 * 1024,   // 50MB
		100 * 1024 * 1024,  // 100MB
		200 * 1024 * 1024,  // 200MB
	}

	for _, size := range sizes {
		testFile := createTestFile(t, size)
		defer os.Remove(testFile)

		// 单线程
		start1 := time.Now()
		hash1 := MD5Check(testFile)
		duration1 := time.Since(start1)

		// 并发（4 workers）
		start2 := time.Now()
		hash2, _ := MD5CheckConcurrent(testFile, 4)
		duration2 := time.Since(start2)

		speedup := float64(duration1) / float64(duration2)

		t.Logf("\n文件大小: %d MB", size/(1024*1024))
		t.Logf("单线程: %v (hash: %s)", duration1, hash1[:8])
		t.Logf("并发4线程: %v (hash: %s)", duration2, hash2[:8])
		t.Logf("加速比: %.2fx", speedup)
		t.Logf("----------------------------------------")
	}
}

// Benchmark 单线程 MD5
func BenchmarkMD5Check(b *testing.B) {
	testFile := createTestFile(&testing.T{}, 10*1024*1024) // 10MB
	defer os.Remove(testFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MD5Check(testFile)
	}
}

// Benchmark 直接 MD5（不使用 WorkPool）
func BenchmarkMD5Direct(b *testing.B) {
	testFile := createTestFile(&testing.T{}, 10*1024*1024) // 10MB
	defer os.Remove(testFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateMD5Direct(testFile)
	}
}

// Benchmark 并发 MD5
func BenchmarkMD5CheckConcurrent(b *testing.B) {
	testFile := createTestFile(&testing.T{}, 10*1024*1024) // 10MB
	defer os.Remove(testFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MD5CheckConcurrent(testFile, 4)
	}
}

// 内存对比测试：直接 MD5 vs WorkPool MD5
func TestMemoryComparison(t *testing.T) {
	// 创建 1GB 测试文件
	testFile := createTestFile(t, 1024*1024*1024)
	defer os.Remove(testFile)

	t.Log("\n========== 内存消耗对比测试 (1GB 文件) ==========")

	// 测试 1: 直接 MD5（不使用 WorkPool）
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var memStatsBefore1, memStatsAfter1 runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore1)
	
	start1 := time.Now()
	hash1, err1 := calculateMD5Direct(testFile)
	duration1 := time.Since(start1)
	
	runtime.ReadMemStats(&memStatsAfter1)

	if err1 != nil {
		t.Fatal(err1)
	}

	allocDiff1 := memStatsAfter1.Alloc - memStatsBefore1.Alloc
	totalAllocDiff1 := memStatsAfter1.TotalAlloc - memStatsBefore1.TotalAlloc

	t.Logf("\n【方法1：直接 MD5 (io.Copy)】")
	t.Logf("  耗时: %v", duration1)
	t.Logf("  Hash: %s", hash1)
	t.Logf("  当前堆内存增量: %.2f KB", float64(allocDiff1)/1024)
	t.Logf("  累计分配内存: %.2f KB", float64(totalAllocDiff1)/1024)
	t.Logf("  分配次数: %d", memStatsAfter1.Mallocs-memStatsBefore1.Mallocs)

	// 测试 2: WorkPool MD5
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var memStatsBefore2, memStatsAfter2 runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore2)
	
	start2 := time.Now()
	hash2, err2 := MD5CheckConcurrent(testFile, 4)
	duration2 := time.Since(start2)
	
	runtime.ReadMemStats(&memStatsAfter2)

	if err2 != nil {
		t.Fatal(err2)
	}

	allocDiff2 := memStatsAfter2.Alloc - memStatsBefore2.Alloc
	totalAllocDiff2 := memStatsAfter2.TotalAlloc - memStatsBefore2.TotalAlloc

	t.Logf("\n【方法2：WorkPool MD5 (4 workers)】")
	t.Logf("  耗时: %v", duration2)
	t.Logf("  Hash: %s", hash2)
	t.Logf("  当前堆内存增量: %.2f MB", float64(allocDiff2)/(1024*1024))
	t.Logf("  累计分配内存: %.2f MB", float64(totalAllocDiff2)/(1024*1024))
	t.Logf("  分配次数: %d", memStatsAfter2.Mallocs-memStatsBefore2.Mallocs)

	// 对比结果
	t.Logf("\n========== 对比结果 ==========")
	speedup := float64(duration1) / float64(duration2)
	memRatio := float64(totalAllocDiff2) / float64(totalAllocDiff1)
	allocRatio := float64(memStatsAfter2.Mallocs-memStatsBefore2.Mallocs) / float64(memStatsAfter1.Mallocs-memStatsBefore1.Mallocs)
	
	t.Logf("  速度对比: %.2fx (%s)", speedup, func() string {
		if speedup > 1 {
			return "WorkPool 更快"
		} else if speedup < 1 {
			return "Direct 更快"
		}
		return "基本相同"
	}())
	t.Logf("  内存消耗: WorkPool 使用 %.2fx 内存 (%.2f MB vs %.2f KB)", 
		memRatio, 
		float64(totalAllocDiff2)/(1024*1024),
		float64(totalAllocDiff1)/1024)
	t.Logf("  分配次数: WorkPool %.2fx", allocRatio)
	
	// 验证 hash 一致性
	if hash1 != hash2 {
		t.Errorf("Hash 不一致！Direct: %s, WorkPool: %s", hash1, hash2)
	} else {
		t.Logf("  ✓ Hash 验证通过")
	}
	
	t.Logf("\n结论：当前 WorkPool 实现是顺序执行，没有真正的并发优势")
}

// 批量 MD5 测试：多个文件并发计算
func TestBatchMD5(t *testing.T) {
	// 创建 10 个 50MB 的测试文件
	fileCount := 10
	fileSize := int64(50 * 1024 * 1024)
	testFiles := make([]string, fileCount)
	
	t.Logf("创建 %d 个 50MB 测试文件...", fileCount)
	for i := 0; i < fileCount; i++ {
		testFiles[i] = createTestFile(t, fileSize)
		defer os.Remove(testFiles[i])
	}
	
	t.Log("\n========== 批量 MD5 计算对比测试 ==========")
	
	// 测试 1: 顺序计算（不使用 WorkPool）
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var memStatsBefore1, memStatsAfter1 runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore1)
	
	start1 := time.Now()
	results1 := make(map[string]string)
	for _, file := range testFiles {
		hash, _ := calculateMD5Direct(file)
		results1[file] = hash
	}
	duration1 := time.Since(start1)
	
	runtime.ReadMemStats(&memStatsAfter1)
	
	totalAllocDiff1 := memStatsAfter1.TotalAlloc - memStatsBefore1.TotalAlloc
	
	t.Logf("\n【方法1：顺序计算】")
	t.Logf("  耗时: %v", duration1)
	t.Logf("  累计分配内存: %.2f KB", float64(totalAllocDiff1)/1024)
	t.Logf("  分配次数: %d", memStatsAfter1.Mallocs-memStatsBefore1.Mallocs)
	
	// 测试 2: WorkPool 并发计算
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var memStatsBefore2, memStatsAfter2 runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore2)
	
	start2 := time.Now()
	results2, err := MD5CheckBatch(testFiles, 4)
	duration2 := time.Since(start2)
	
	runtime.ReadMemStats(&memStatsAfter2)
	
	if err != nil {
		t.Fatal(err)
	}
	
	totalAllocDiff2 := memStatsAfter2.TotalAlloc - memStatsBefore2.TotalAlloc
	
	t.Logf("\n【方法2：WorkPool 并发 (4 workers)】")
	t.Logf("  耗时: %v", duration2)
	t.Logf("  累计分配内存: %.2f KB", float64(totalAllocDiff2)/1024)
	t.Logf("  分配次数: %d", memStatsAfter2.Mallocs-memStatsBefore2.Mallocs)
	
	// 对比结果
	t.Logf("\n========== 对比结果 ==========")
	speedup := float64(duration1) / float64(duration2)
	memRatio := float64(totalAllocDiff2) / float64(totalAllocDiff1)
	
	t.Logf("  速度提升: %.2fx", speedup)
	t.Logf("  内存消耗: %.2fx", memRatio)
	
	// 验证结果一致性
	allMatch := true
	for file := range results1 {
		if results1[file] != results2[file] {
			t.Errorf("文件 %s Hash 不一致！", file)
			allMatch = false
		}
	}
	
	if allMatch {
		t.Logf("  ✓ 所有文件 Hash 验证通过")
	}
	
	t.Logf("\n结论：WorkPool 在批量处理多个文件时才有真正的并发优势")
}

// Benchmark 对比：直接 MD5 vs WorkPool MD5
func BenchmarkMD5Comparison(b *testing.B) {
	testFile := createTestFile(&testing.T{}, 10*1024*1024) // 10MB
	defer os.Remove(testFile)

	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			calculateMD5Direct(testFile)
		}
	})

	b.Run("WorkPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			MD5CheckConcurrent(testFile, 4)
		}
	})
}

// 测试不同 worker 数量的性能
func TestMD5WorkerCount(t *testing.T) {
	testFile := createTestFile(t, 100*1024*1024) // 100MB
	defer os.Remove(testFile)

	workerCounts := []int{1, 2, 4, 8, 16}

	t.Logf("\n100MB 文件，不同 worker 数量的性能对比：")
	t.Logf("----------------------------------------")

	for _, count := range workerCounts {
		start := time.Now()
		hash, _ := MD5CheckConcurrent(testFile, count)
		duration := time.Since(start)

		t.Logf("Workers: %2d | 耗时: %v | Hash: %s", count, duration, hash[:8])
	}
}
