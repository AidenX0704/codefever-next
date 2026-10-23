package handlers

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

// renderQRCode 把文本渲染为 SVG 二维码。
//
// 为什么自己渲染：PHP 侧用 Endroid QrCode 的 SvgWriter，
// 契约里 /api/user/getMFAData 返回的 image 字段是
// `data:image/svg+xml;charset=utf-8;base64,<svg>`；
// 而 Go 的 skip2/go-qrcode 只提供 PNG 与位图，没有 SVG 输出。
//
// 为了不改变契约（前端 <img src={data}> 直接吃这个 data URI），
// 这里基于 QR 位图自行生成 SVG：白色背景 + 黑色模块矩形。
//
// 与 PHP 版的差异：
//   - 尺寸同为 300，纠错等级同为 L；
//   - 静默区（quiet zone）宽度由 go-qrcode 决定（4 个模块），
//     PHP 版用 setMargin(10) + RoundBlockSizeModeMargin；
//     因此图像像素不完全一致，但**编码内容完全相同**，
//     认证器 App 的扫码结果一致。
func renderQRCode(content string) ([]byte, error) {
	q, err := qrcode.New(content, qrcode.Low)
	if err != nil {
		return nil, fmt.Errorf("生成二维码失败: %w", err)
	}

	bits := q.Bitmap()
	n := len(bits)
	if n == 0 {
		return nil, fmt.Errorf("二维码位图为空")
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300" viewBox="0 0 `)
	fmt.Fprintf(&sb, "%d %d", n, n)
	sb.WriteString(`" shape-rendering="crispEdges">`)
	sb.WriteString(`<rect width="100%" height="100%" fill="#FFFFFF"/>`)
	sb.WriteString(`<path fill="#000000" d="`)

	// 把连续的黑色模块合并成水平段，减少 path 体积。
	for y := 0; y < n; y++ {
		x := 0
		for x < n {
			if !bits[y][x] {
				x++
				continue
			}
			start := x
			for x < n && bits[y][x] {
				x++
			}
			fmt.Fprintf(&sb, "M%d %dh%dv1h-%dz", start, y, x-start, x-start)
		}
	}
	sb.WriteString(`"/>`)
	sb.WriteString(`</svg>`)

	return []byte(sb.String()), nil
}

// base64Std 是 base64 标准编码的薄封装。
func base64Std(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
