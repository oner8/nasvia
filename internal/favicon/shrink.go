package favicon

import (
	"bytes"
	"image"
	_ "image/gif"  // 注册 GIF 解码
	_ "image/jpeg" // 注册 JPEG 解码
	"image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // 注册 WebP 解码
)

// MaxIconSize 落盘图标的最大边长（像素）。首页图标最大约 64px 显示，192 足够 3 倍屏清晰；
// HD-Icons 原图是 1024×1024（单张可达 300KB），缩到 192 后通常只有十几 KB，
// 经反代 / 内网穿透访问时能大幅节省流量、加快首屏。
const MaxIconSize = 192

// Shrink 把边长超过 maxSize 的位图（PNG / JPEG / GIF / WebP）等比缩小并重新编码为 PNG。
// 无需缩小、无法解码（ICO、SVG 等）或缩小后反而更大时返回 ok=false，调用方继续用原图。
func Shrink(data []byte, maxSize int) (out []byte, contentType string, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (cfg.Width <= maxSize && cfg.Height <= maxSize) || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", false
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", false
	}
	w, h := maxSize, maxSize
	if cfg.Width > cfg.Height {
		h = max(1, cfg.Height*maxSize/cfg.Width)
	} else if cfg.Height > cfg.Width {
		w = max(1, cfg.Width*maxSize/cfg.Height)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, dst); err != nil || buf.Len() >= len(data) {
		return nil, "", false
	}
	return buf.Bytes(), "image/png", true
}
