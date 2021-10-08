package rolling

import (
	"log"
	"testing"
)

func TestExample(t *testing.T) {
	writer, _ := NewWriter(
		WithLogPath("E:\\public\\public_project\\files\\log"),
		WithFilename("all.log"),
		WithMaxRemain(100), // 保留 10 个文件
		WithMaxSize(3),     // 每个文件最大为 10M
		WithMaxAge(30),     // 保留天数
		WithCompress(),
		WithLocalTime(),
		WithTimeRolling(),
	)
	log.SetOutput(writer)
	log.Println("Hello World")
}
