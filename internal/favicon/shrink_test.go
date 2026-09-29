package favicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestShrinkLargeIcon(t *testing.T) {
	src := pngOf(t, 1024, 512)
	out, ct, ok := Shrink(src, MaxIconSize)
	if !ok || ct != "image/png" {
		t.Fatalf("大图应被缩小：ok=%v ct=%q", ok, ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != MaxIconSize || cfg.Height != MaxIconSize/2 {
		t.Fatalf("应等比缩到 %dx%d，得到 %dx%d", MaxIconSize, MaxIconSize/2, cfg.Width, cfg.Height)
	}
	if len(out) >= len(src) {
		t.Fatalf("缩小后应更小：%d >= %d", len(out), len(src))
	}
}

func TestShrinkSkipsSmallAndUndecodable(t *testing.T) {
	if _, _, ok := Shrink(pngOf(t, 64, 64), MaxIconSize); ok {
		t.Fatal("小图不应处理")
	}
	if _, _, ok := Shrink([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), MaxIconSize); ok {
		t.Fatal("SVG 不应处理")
	}
	if _, _, ok := Shrink([]byte("\x00\x00\x01\x00garbage"), MaxIconSize); ok {
		t.Fatal("ICO / 无法解码的数据不应处理")
	}
}
