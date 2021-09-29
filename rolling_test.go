package rolling

import (
	"fmt"
	"testing"
	"time"
)

func TestWrite(t *testing.T) {
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
	_, _ = fmt.Fprintf(writer, "now :%s \n", time.Now().Format("2006-01-02T15-04-05.000"))
	//wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		go func(int) {
			for {
				_, err := fmt.Fprintf(writer, "now :%s \n", time.Now().Format("2006-01-02T15-04-05.000"))
				if err != nil {
					return
				}
			}
		}(i)
	}
	select {}
	//wg.Wait()
}
