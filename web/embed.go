// Package webui 内嵌前端构建产物（web/dist）。
//
// 构建顺序：先在 web/ 目录执行 npm run build 生成 dist，再编译 Go 二进制，
// 这样单个二进制即可同时提供 API 与页面。
package webui

import "embed"

// Dist 内嵌的前端静态资源。
//
//go:embed all:dist
var Dist embed.FS
