// Package favicon 负责抓取、嗅探、缓存站点图标，并在全部来源失败时生成占位图标。
//
// 回退链：站点 /favicon.ico → 解析首页 <link rel=icon> → DuckDuckGo → Google S2 → 内置占位 SVG。
package favicon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxIconBytes = 1 << 20 // 1MiB
	maxPageBytes = 512 << 10
	// stepTimeout 限制单个回退步骤（一次 HTTP 尝试）的耗时上限。
	stepTimeout = 7 * time.Second
	userAgent   = "NASVIA/0.1 (+favicon fetcher)"
)

// Sources 抓取来源开关；HDIcons → NASIcon（中文名/域名兜底）→ site → duckduckgo → google 依次回退。
type Sources struct {
	Site       bool `json:"site"`
	HDIcons    bool `json:"hdicons"`
	NASIcon    bool `json:"nasicon"`
	DuckDuckGo bool `json:"duckduckgo"`
	Google     bool `json:"google"`
}

// ParseSources 解析 "hdicons,nasicon,site,duckduckgo,google" 形式的来源列表。
func ParseSources(raw string) Sources {
	out := Sources{}
	for _, part := range strings.Split(raw, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "site", "self":
			out.Site = true
		case "hdicons", "hd-icons", "hdicon":
			out.HDIcons = true
		case "nasicon", "nas-icons", "nasicons", "nas-icon":
			out.NASIcon = true
		case "duckduckgo", "ddg":
			out.DuckDuckGo = true
		case "google", "google-s2", "s2":
			out.Google = true
		}
	}
	return out
}

// String 序列化来源列表（hdicons 在最前、nasicon 紧随其后，表示优先级最高）。
func (s Sources) String() string {
	parts := make([]string, 0, 5)
	if s.HDIcons {
		parts = append(parts, "hdicons")
	}
	if s.NASIcon {
		parts = append(parts, "nasicon")
	}
	if s.Site {
		parts = append(parts, "site")
	}
	if s.DuckDuckGo {
		parts = append(parts, "duckduckgo")
	}
	if s.Google {
		parts = append(parts, "google")
	}
	return strings.Join(parts, ",")
}

// Any 是否启用了任一来源。
func (s Sources) Any() bool { return s.Site || s.HDIcons || s.NASIcon || s.DuckDuckGo || s.Google }

// Result 抓取结果。
type Result struct {
	Data        []byte
	ContentType string
	Source      string
	Domain      string
}

// Fetcher 图标抓取器；可并发使用。
type Fetcher struct {
	mu      sync.RWMutex
	sources Sources
	client  *http.Client
}

// NewFetcher 创建抓取器。
func NewFetcher(sources Sources) *Fetcher {
	return &Fetcher{
		sources: sources,
		client: &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// Sources 返回当前来源配置。
func (f *Fetcher) Sources() Sources {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.sources
}

// SetSources 更新来源配置。
func (f *Fetcher) SetSources(s Sources) {
	f.mu.Lock()
	f.sources = s
	f.mu.Unlock()
}

// NormalizeDomain 从 URL 或裸域名中提取小写主机名。
func NormalizeDomain(raw string) (string, error) {
	_, domain, _, err := normalizeTarget(raw)
	return domain, err
}

func normalizeTarget(raw string) (base *url.URL, domain string, scheme string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", "", errors.New("empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, "", "", fmt.Errorf("missing host in %q", raw)
	}
	base = &url.URL{Scheme: u.Scheme, Host: u.Host}
	return base, strings.ToLower(host), u.Scheme, nil
}

// Fetch 按回退链抓取图标；全部失败时返回错误（调用方负责回落到占位图标）。
func (f *Fetcher) Fetch(ctx context.Context, raw string) (*Result, error) {
	base, domain, _, err := normalizeTarget(raw)
	if err != nil {
		return nil, err
	}
	src := f.Sources()
	var errs []error

	// 每个回退步骤都有独立超时：某个来源卡住时不会拖垮整条链，
	// 从而保证「抓不到也要及时回落到下一个来源 / 占位图标」。
	fetch := func(target, source string) (*Result, error) {
		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		defer cancel()
		return f.fetchImage(stepCtx, target, source)
	}
	discover := func() ([]string, error) {
		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		defer cancel()
		return f.discoverIcons(stepCtx, base)
	}

	if src.Site {
		icoURL := fmt.Sprintf("%s://%s/favicon.ico", base.Scheme, base.Host)
		if res, err := fetch(icoURL, "site-ico"); err == nil {
			res.Domain = domain
			return res, nil
		} else {
			errs = append(errs, fmt.Errorf("favicon.ico: %w", err))
		}

		links, err := discover()
		if err != nil {
			errs = append(errs, err)
		}
		for _, link := range links {
			if res, err := fetch(link, "site-html"); err == nil {
				res.Domain = domain
				return res, nil
			} else {
				errs = append(errs, err)
			}
		}
	}
	if src.DuckDuckGo {
		target := fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", domain)
		if res, err := fetch(target, "duckduckgo"); err == nil {
			res.Domain = domain
			return res, nil
		} else {
			errs = append(errs, fmt.Errorf("duckduckgo: %w", err))
		}
	}
	if src.Google {
		target := "https://www.google.com/s2/favicons?sz=64&domain=" + url.QueryEscape(domain)
		if res, err := fetch(target, "google-s2"); err == nil {
			res.Domain = domain
			return res, nil
		} else {
			errs = append(errs, fmt.Errorf("google-s2: %w", err))
		}
	}
	if len(errs) == 0 {
		errs = append(errs, errors.New("no favicon source enabled"))
	}
	if domain != "" {
		errs = append(errs, fmt.Errorf("domain=%s", domain))
	}
	return nil, errors.Join(errs...)
}

func (f *Fetcher) discoverIcons(ctx context.Context, base *url.URL) ([]string, error) {
	pageURL := fmt.Sprintf("%s://%s/", base.Scheme, base.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("homepage %s: %w", pageURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("homepage %s: HTTP %d", pageURL, resp.StatusCode)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "html") {
		return nil, fmt.Errorf("homepage %s: unexpected content-type %q", pageURL, ct)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	if err != nil {
		return nil, fmt.Errorf("homepage %s: %w", pageURL, err)
	}
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	return ParseIconLinks(body, parsed), nil
}

var (
	linkTagRe = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attrRe    = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	sizeNumRe = regexp.MustCompile(`(\d+)`)
)

// ParseIconLinks 从 HTML 中提取图标候选地址，按“可能更好看”的顺序返回，最多 5 条。
func ParseIconLinks(page []byte, pageURL *url.URL) []string {
	html := string(page)
	type candidate struct {
		url      string
		priority int
	}
	var items []candidate
	seen := map[string]bool{}

	for _, tag := range linkTagRe.FindAllString(html, -1) {
		attrs := parseAttrs(tag)
		rel := strings.ToLower(attrs["rel"])
		if rel == "" || !strings.Contains(rel, "icon") {
			continue
		}
		href := strings.TrimSpace(attrs["href"])
		if href == "" || strings.HasPrefix(strings.ToLower(href), "data:") {
			continue
		}
		ref, err := url.Parse(href)
		if err != nil {
			continue
		}
		abs := pageURL.ResolveReference(ref)
		if abs.Scheme != "http" && abs.Scheme != "https" {
			continue
		}
		key := abs.String()
		if seen[key] {
			continue
		}
		seen[key] = true

		priority := 0
		if strings.Contains(rel, "apple-touch-icon") {
			priority += 1000
		}
		if strings.Contains(rel, "mask-icon") {
			priority -= 500
		}
		for _, m := range sizeNumRe.FindAllStringSubmatch(attrs["sizes"], -1) {
			if n, err := strconv.Atoi(m[1]); err == nil {
				priority += n
			}
		}
		items = append(items, candidate{url: key, priority: priority})
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].priority > items[j].priority })
	out := make([]string, 0, len(items))
	for _, it := range items {
		if len(out) >= 5 {
			break
		}
		out = append(out, it.url)
	}
	return out
}

func parseAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
		key := strings.ToLower(m[1])
		value := m[2]
		if value == "" {
			value = m[3]
		}
		if value == "" {
			value = m[4]
		}
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	return out
}

func (f *Fetcher) fetchImage(ctx context.Context, rawURL, source string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s -> HTTP %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIconBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s -> empty body", rawURL)
	}
	if len(data) > maxIconBytes {
		return nil, fmt.Errorf("%s -> body exceeds %d bytes", rawURL, maxIconBytes)
	}
	ct, ok := SniffImage(data, resp.Header.Get("Content-Type"))
	if !ok {
		return nil, fmt.Errorf("%s -> not an image", rawURL)
	}
	return &Result{Data: data, ContentType: ct, Source: source}, nil
}

// SniffImage 通过魔数（必要时回退到响应头）判断内容是否为图片。
func SniffImage(data []byte, headerCT string) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return "image/jpeg", true
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif", true
	case len(data) > 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", true
	case bytes.HasPrefix(data, []byte("\x00\x00\x01\x00")), bytes.HasPrefix(data, []byte("\x00\x00\x02\x00")):
		return "image/x-icon", true
	}
	head := bytes.TrimSpace(data)
	if len(head) > 512 {
		head = head[:512]
	}
	lower := bytes.ToLower(head)
	if bytes.Contains(lower, []byte("<svg")) && !bytes.Contains(lower, []byte("<html")) && !bytes.Contains(lower, []byte("<!doctype html")) {
		return "image/svg+xml", true
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(headerCT, ";")[0]))
	if strings.HasPrefix(ct, "image/") && !strings.Contains(ct, "svg") {
		return ct, true
	}
	if ct == "image/svg+xml" {
		return ct, true
	}
	return "", false
}

// Hash 返回内容哈希（前 32 位十六进制），用于图标文件名。
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:32]
}

// Ext 返回与内容类型匹配的文件扩展名。
func Ext(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/x-icon", "image/vnd.microsoft.icon", "image/ico":
		return ".ico"
	default:
		return ".img"
	}
}

// Placeholder 生成确定性的自绘占位图标（首字母 + 由名称派生的渐变色）。
func Placeholder(name string, size int) []byte {
	if size <= 0 {
		size = 96
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	hue := int(sum[0])*360/256 + int(sum[1])%24
	hue2 := (hue + 42) % 360
	letter := "?"
	for _, r := range strings.TrimSpace(name) {
		letter = strings.ToUpper(string(r))
		break
	}
	letter = escapeXML(letter)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 96 96" role="img" aria-label="%s">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%%" stop-color="hsl(%d 72%% 55%%)"/>
      <stop offset="100%%" stop-color="hsl(%d 68%% 42%%)"/>
    </linearGradient>
  </defs>
  <rect width="96" height="96" rx="24" fill="url(#g)"/>
  <text x="48" y="49" text-anchor="middle" dominant-baseline="central" font-family="system-ui,-apple-system,'Segoe UI',sans-serif" font-size="44" font-weight="600" fill="#fff">%s</text>
</svg>`, size, size, letter, hue, hue2, letter)
	return []byte(svg)
}

func escapeXML(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(s)
}

// TrimFrame 裁掉图片最外圈的纯色细边框，返回新数据；没有可裁的边框时返回 nil。
//
// 起因：nasicon.top 里有些条目其实是网页截图，最外圈带一圈 1px 灰线（深色主题下像黑边）。
// 判定很保守，只在同时满足时才裁，最多 3px：
//  1. 最外圈的不透明像素是同一个颜色（圆角处的透明像素忽略）；
//  2. 紧靠它内侧的不透明像素与这个颜色明显不同（任一通道差异 > 24/255）。
//
// 所以自带背景色的正常图标（HD-Icons 那种圆角方块、白底方形图标、周围留透明的 favicon）
// 都不会被误裁——它们的「外圈颜色」和「内侧颜色」是一样的。
func TrimFrame(data []byte) []byte {
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return nil // 只处理 PNG：JPEG 重编码有损，其它格式原样保留
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 32 || height < 32 {
		return nil // 太小，谈不上边框
	}
	trim := frameWidth(img, bounds)
	if trim <= 0 {
		return nil
	}
	cropped := image.NewRGBA(image.Rect(0, 0, width-2*trim, height-2*trim))
	draw.Draw(cropped, cropped.Bounds(), img, bounds.Min.Add(image.Pt(trim, trim)), draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil
	}
	return buf.Bytes()
}

// frameWidth 返回最外圈纯色边框的厚度（0 表示没有）。
// 从厚到薄找：取最大的 thickness，使「距边界 thickness 以内这一圈颜色一致」且「再往里一带明显不同」。
func frameWidth(img image.Image, bounds image.Rectangle) int {
	const maxTrim = 3
	const innerBand = 3
	for thickness := maxTrim; thickness >= 1; thickness-- {
		ring, ok := uniformRingColor(img, bounds, thickness)
		if !ok {
			continue
		}
		if innerDiffers(img, bounds, thickness, innerBand, ring) {
			return thickness
		}
	}
	return 0
}

type rgba struct{ r, g, b uint8 }

// uniformRingColor 取「距边界 thickness 像素以内」这一圈的主色；
// 主色占比不足 30%（或整圈都透明）时返回 ok=false。
// 圆角处的抗锯齿像素会让颜色轻微漂移，所以判定时把「与主色接近」的像素一并计入，
// 合计仍不足 80% 就说明外圈不是单一颜色（渐变、图片内容等），不算边框。
func uniformRingColor(img image.Image, bounds image.Rectangle, thickness int) (rgba, bool) {
	counts := map[rgba]int{}
	total := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if edgeDistance(x, y, bounds) >= thickness {
				continue
			}
			r, g, b, a := img.At(x, y).RGBA()
			if a/257 < 200 {
				continue // 圆角处的透明像素不参与判定
			}
			counts[rgba{uint8(r / 257), uint8(g / 257), uint8(b / 257)}]++
			total++
		}
	}
	if total == 0 {
		return rgba{}, false
	}
	var top rgba
	topCount := 0
	for color, count := range counts {
		if count > topCount {
			top, topCount = color, count
		}
	}
	if topCount*100 < total*30 {
		return rgba{}, false // 杂色太多，这不是纯色边框
	}
	// 圆角处的抗锯齿像素会把主色稍微拉偏，所以把「接近主色」的像素一起算进来；
	// 合计占比仍不足 80% 就说明外圈不是单一颜色（例如渐变或图片内容），不裁。
	close := 0
	for color, count := range counts {
		if diff(top, color) <= ringColorTolerance {
			close += count
		}
	}
	if close*100 < total*80 {
		return rgba{}, false
	}
	return top, true
}

// ringColorTolerance 判定「与边框主色属于同一种颜色」的抗锯齿容差（任一通道，0-255）。
const ringColorTolerance = 32

// innerDiffers 判断 thickness..thickness+band 这一带里，是否有至少一半的不透明像素
// 与边框主色明显不同（任一通道差异 > frameDiffThreshold）——
// 用来确认这圈真的是「边框」，而不是图标自己的底色。
func innerDiffers(img image.Image, bounds image.Rectangle, thickness, band int, ring rgba) bool {
	differ, opaque := 0, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			edge := edgeDistance(x, y, bounds)
			if edge < thickness || edge >= thickness+band {
				continue
			}
			r, g, b, a := img.At(x, y).RGBA()
			if a/257 < 200 {
				continue
			}
			opaque++
			if diff(ring, rgba{uint8(r / 257), uint8(g / 257), uint8(b / 257)}) > frameDiffThreshold {
				differ++
			}
		}
	}
	return opaque > 0 && differ*2 >= opaque
}

// edgeDistance 像素到图片边界的距离（四边取最小）。
func edgeDistance(x, y int, bounds image.Rectangle) int {
	return min(x-bounds.Min.X, bounds.Max.X-1-x, y-bounds.Min.Y, bounds.Max.Y-1-y)
}

// frameDiffThreshold 边框色与内侧色的最小差异（任一通道，0-255）。
const frameDiffThreshold = 24

// diff 两色在任一通道上的最大差异。
func diff(a, b rgba) int {
	return max(absInt(int(a.r)-int(b.r)), absInt(int(a.g)-int(b.g)), absInt(int(a.b)-int(b.b)))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
