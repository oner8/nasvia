package favicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func pngBytes() []byte {
	// 最小 PNG 头，足够通过魔数嗅探
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
}

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"https://Jellyfin.Home.Example.com:8096/web": "jellyfin.home.example.com",
		"example.com":              "example.com",
		"http://192.168.1.10:8096": "192.168.1.10",
	}
	for input, want := range cases {
		got, err := NormalizeDomain(input)
		if err != nil {
			t.Fatalf("NormalizeDomain(%q) 出错：%v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeDomain(%q) = %q, want %q", input, got, want)
		}
	}
	for _, bad := range []string{"", "ftp://example.com", "https://"} {
		if _, err := NormalizeDomain(bad); err == nil {
			t.Fatalf("NormalizeDomain(%q) 应该报错", bad)
		}
	}
}

func TestParseIconLinks(t *testing.T) {
	page := []byte(`<html><head>
		<link rel="icon" href="/favicon-32.png" sizes="32x32">
		<link rel="apple-touch-icon" sizes="180x180" href="https://cdn.example.com/apple-touch.png">
		<link rel="mask-icon" href="/mask.svg">
		<link rel="stylesheet" href="/style.css">
		<link rel="icon" href="data:image/png;base64,AAAA">
		<link rel="shortcut icon" href="/favicon.ico">
	</head></html>`)

	base, err := url.Parse("https://example.com/some/page")
	if err != nil {
		t.Fatal(err)
	}
	links := ParseIconLinks(page, base)
	if len(links) == 0 {
		t.Fatal("应至少解析出一个图标地址")
	}
	if links[0] != "https://cdn.example.com/apple-touch.png" {
		t.Fatalf("apple-touch-icon 应排在最前，实际：%v", links)
	}
	joined := strings.Join(links, " ")
	for _, want := range []string{"https://example.com/favicon-32.png", "https://example.com/favicon.ico"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺少候选 %s，实际：%v", want, links)
		}
	}
	if strings.Contains(joined, "data:") {
		t.Fatalf("data: 图标应被忽略：%v", links)
	}
	if strings.Contains(joined, "style.css") {
		t.Fatalf("非 icon 的 link 不应被当作图标：%v", links)
	}
}

func TestSniffImage(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		ct   string
		want string
		ok   bool
	}{
		{"png", pngBytes(), "", "image/png", true},
		{"jpeg", []byte("\xff\xd8\xff\xe0rest"), "", "image/jpeg", true},
		{"gif", []byte("GIF89a....."), "", "image/gif", true},
		{"webp", append([]byte("RIFF\x00\x00\x00\x00WEBP"), 'V'), "", "image/webp", true},
		{"ico", []byte("\x00\x00\x01\x00\x01\x00"), "", "image/x-icon", true},
		{"svg", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "", "image/svg+xml", true},
		{"html", []byte("<html><body>hi<svg/></body></html>"), "text/html", "", false},
		{"plain", []byte("hello world"), "text/plain", "", false},
		{"header-only", []byte{0x01, 0x02, 0x03}, "image/png; charset=binary", "image/png", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SniffImage(tc.data, tc.ct)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("SniffImage = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestExtAndHash(t *testing.T) {
	if Ext("image/png") != ".png" || Ext("image/svg+xml; charset=utf-8") != ".svg" || Ext("weird") != ".img" {
		t.Fatal("Ext 映射不正确")
	}
	a := Hash(pngBytes())
	b := Hash(pngBytes())
	if a != b || len(a) != 32 {
		t.Fatalf("Hash 应为确定性的 32 位十六进制：%s / %s", a, b)
	}
	if Hash([]byte("x")) == a {
		t.Fatal("不同内容应得到不同哈希")
	}
}

func TestPlaceholderIsDeterministicSVG(t *testing.T) {
	first := string(Placeholder("Jellyfin", 96))
	second := string(Placeholder("Jellyfin", 96))
	if first != second {
		t.Fatal("同一名称应生成相同的占位图标")
	}
	if !strings.Contains(first, "<svg") || !strings.Contains(first, "</svg>") {
		t.Fatalf("占位图标应为合法 SVG：%s", first)
	}
	if first == string(Placeholder("Immich", 96)) {
		t.Fatal("不同名称应生成不同的占位图标")
	}
	escaped := string(Placeholder(`<script>`, 96))
	if strings.Contains(escaped, "<script>") {
		t.Fatal("首字母应被 XML 转义")
	}
}

func TestFetchFallsBackToHTMLIcon(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/icon.png"></head></html>`))
	})
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes())
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	fetcher := NewFetcher(Sources{Site: true})
	res, err := fetcher.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("抓取失败：%v", err)
	}
	if res.Source != "site-html" {
		t.Fatalf("应回退到 HTML 图标，实际来源 %q", res.Source)
	}
	if res.ContentType != "image/png" {
		t.Fatalf("内容类型错误：%s", res.ContentType)
	}
	if res.Domain == "" {
		t.Fatal("应记录域名")
	}
}

func TestFetchAllSourcesFail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head></head></html>"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	fetcher := NewFetcher(Sources{Site: true})
	if _, err := fetcher.Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("全部来源失败时应返回错误，由调用方回落到占位图标")
	}

	disabled := NewFetcher(Sources{})
	if _, err := disabled.Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("未启用任何来源时应返回错误")
	}
}

func TestSourcesParseAndString(t *testing.T) {
	src := ParseSources("site, duckduckgo ,google")
	if !src.Any() || !src.Site || !src.DuckDuckGo || !src.Google {
		t.Fatalf("解析结果不正确：%+v", src)
	}
	if got := ParseSources("").String(); got != "" {
		t.Fatalf("空来源应序列化为空串，得到 %q", got)
	}
	if got := ParseSources("duckduckgo").String(); got != "duckduckgo" {
		t.Fatalf("序列化错误：%q", got)
	}
}

var _ = binary.BigEndian

// makeIcon 生成一张 size×size 的测试图：fill 填充，最外 ringWidth 像素用 border 色，
// 四角按 22% 比例裁圆（透明）——模拟真实的圆角图标/截图。
func makeIcon(t *testing.T, size int, fill, border color.RGBA, ringWidth int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	radius := int(float64(size) * 0.22)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !insideRounded(x, y, size, radius) {
				img.Set(x, y, color.RGBA{})
				continue
			}
			if edge := min(x, size-1-x, y, size-1-y); edge < ringWidth {
				img.Set(x, y, border)
				continue
			}
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func insideRounded(x, y, size, radius int) bool {
	if (x < radius || x >= size-radius) && (y < radius || y >= size-radius) {
		cx, cy := radius, radius
		if x >= size-radius {
			cx = size - 1 - radius
		}
		if y >= size-radius {
			cy = size - 1 - radius
		}
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= radius*radius
	}
	return true
}

func TestTrimFrame(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	gray := color.RGBA{179, 179, 179, 255}
	navy := color.RGBA{20, 20, 60, 255}

	// 1) 白底 + 1px 灰边（西瓜视频/腾讯视频那种网页截图）→ 裁掉 1px，裁后边缘变白
	out := TrimFrame(makeIcon(t, 64, white, gray, 1))
	if out == nil {
		t.Fatal("带 1px 灰边的截图应被裁掉边框")
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("裁剪结果解码失败: %v", err)
	}
	if got := img.Bounds().Dx(); got != 62 {
		t.Errorf("裁剪后宽度应为 62，得到 %d", got)
	}
	if r, g, b, a := img.At(31, 0).RGBA(); r/257 != 255 || g/257 != 255 || b/257 != 255 || a/257 < 200 {
		t.Errorf("裁剪后最外圈应为白色不透明，得到 (%d,%d,%d a=%d)", r/257, g/257, b/257, a/257)
	}

	// 2) 3px 边框也要能整条裁掉
	if out := TrimFrame(makeIcon(t, 64, white, gray, 3)); out == nil {
		t.Error("3px 边框应被裁掉")
	} else if img, _ := png.Decode(bytes.NewReader(out)); img.Bounds().Dx() != 58 {
		t.Errorf("3px 边框裁剪后宽度应为 58，得到 %d", img.Bounds().Dx())
	}

	// 3) 自带背景色的圆角方块（HD-Icons 风格，外圈与内侧同色）→ 不动
	if out := TrimFrame(makeIcon(t, 64, navy, navy, 1)); out != nil {
		t.Error("自带背景色的圆角图标不该被裁剪")
	}
	// 4) 纯白底方图（无边框）→ 不动
	if out := TrimFrame(makeIcon(t, 64, white, white, 1)); out != nil {
		t.Error("白底无边框不该被裁剪")
	}
	// 5) 太小 / 非 PNG / 空数据 / 损坏 → 不处理
	if out := TrimFrame(makeIcon(t, 16, white, gray, 1)); out != nil {
		t.Error("过小的图不该处理")
	}
	if out := TrimFrame([]byte("GIF89a............")); out != nil {
		t.Error("非 PNG 应返回 nil")
	}
	if out := TrimFrame(nil); out != nil {
		t.Error("空数据应返回 nil")
	}
	if out := TrimFrame([]byte("\x89PNG\r\n\x1a\nbroken")); out != nil {
		t.Error("损坏的 PNG 应返回 nil")
	}
}
