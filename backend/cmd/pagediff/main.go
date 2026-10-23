// Command pagediff 抓取两个实例的同一路径并逐字节比对，输出差异上下文。
//
// 用法：
//
//	go run ./cmd/pagediff -php http://localhost:9080 -go http://127.0.0.1:9003 /user/register
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	phpBase := flag.String("php", "http://localhost:9080", "PHP 实例地址")
	goBase := flag.String("go", "http://127.0.0.1:9003", "Go 实例地址")
	ctx := flag.Int("ctx", 400, "差异上下文的字节数")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("用法: pagediff <path>")
		os.Exit(2)
	}
	path := flag.Args()[0]

	fetch := func(base string) []byte {
		resp, err := http.Get(base + path)
		if err != nil {
			fmt.Printf("请求 %s 失败: %v\n", base+path, err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return b
	}

	a := fetch(*phpBase)
	b := fetch(*goBase)
	fmt.Printf("php len=%d  go len=%d\n", len(a), len(b))
	if bytes.Equal(a, b) {
		fmt.Println("完全一致")
		return
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	start := i - *ctx
	if start < 0 {
		start = 0
	}
	endA := i + *ctx
	if endA > len(a) {
		endA = len(a)
	}
	endB := i + *ctx
	if endB > len(b) {
		endB = len(b)
	}
	fmt.Printf("首个差异在第 %d 字节\n", i)
	fmt.Printf("--- php ---\n%q\n", a[start:endA])
	fmt.Printf("--- go  ---\n%q\n", b[start:endB])
}
