// Package nasicon 从 nasicon.top（FlatNas 图标集）按「中文名 / 英文名 / 域名」匹配图标，
// 作为 xushier/HD-Icons 之后的兜底来源：HD-Icons 索引里只有英文名，中文站点名几乎配不上；
// 这份索引带 cnName 与 domain，中文名可以精确命中（百度网盘 → Baidunetdisk_A--百度网盘--qdnas-s.png）。
//
// 索引 https://nasicon.top/icons.json 是 {name,cnName,domain,filename,url} 的数组（约 1964 条、350 KB）。
// 匹配刻意保守（与 hdicons 同一思路，宁可配不到也不乱配）：
//  1. 站点名 slug 等于条目 name（忽略末尾的 A/B/C 变体后缀）：无变体优先，其次按字母序取 A、B…；
//  2. 站点名等于 cnName（精确，不做前缀/包含：「百度网盘」不能被配到促销图「百度网盘年卡特惠」）；
//  3. 站点主机的域名等于条目 domain，或主机以 "."+domain 结尾（domain 必须含 "."，滤掉 qdnas-s 这类噪声）。
//
// 绝不做模糊/包含匹配。
package nasicon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oner8/nasvia/internal/hdicons"
)

const (
	// DefaultBase 站点根地址；挂了或想换地址可用 NASVIA_NASICON_BASE 覆盖。
	DefaultBase = "https://nasicon.top"
	// IndexFile 索引文件名（站点根目录下的 JSON 数组）。
	IndexFile = "icons.json"
	// IconDir 图标目录（条目 filename 相对站点根的位置）。
	IconDir = "/icon/"
	// IndexTTL 索引缓存有效期。
	IndexTTL = 7 * 24 * time.Hour

	maxIndexBytes = 4 << 20
	// maxIconBytes 上限放宽到 4MB：这套图里有 1024×1024 甚至更大的 PNG（抽样平均 45KB，已见到 320KB）。
	maxIconBytes = 4 << 20
	requestLimit = 12 * time.Second
)

// Item 索引里的一条图标。
type Item struct {
	Name     string `json:"name"`
	CNName   string `json:"cnName"`
	Domain   string `json:"domain"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// BaseName 去掉末尾的变体后缀：`115 A` → `115`、`Baidunetdisk B` → `Baidunetdisk`。
func (i Item) BaseName() string {
	base, _ := splitVariant(i.Name)
	return base
}

// Variant 变体字母（A/B/C…）；没有则返回 ""。
func (i Item) Variant() string {
	_, variant := splitVariant(i.Name)
	return variant
}

// splitVariant 拆出末尾的单个大写字母变体（前面必须是空格）。
func splitVariant(name string) (string, string) {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) >= 2 {
		last := trimmed[len(trimmed)-1]
		if last >= 'A' && last <= 'Z' && trimmed[len(trimmed)-2] == ' ' {
			return strings.TrimSpace(trimmed[:len(trimmed)-1]), string(last)
		}
	}
	return trimmed, ""
}

// ParseIndex 解析索引 JSON 数组；空或全无 filename 时报错（避免把坏数据写成"成功"的缓存）。
func ParseIndex(data []byte) ([]Item, error) {
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("解析图标索引失败: %w", err)
	}
	out := make([]Item, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Filename) == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, errors.New("图标索引为空")
	}
	return out, nil
}

// ---------------------------------------------------------------- 名称匹配

// Match 返回匹配到的条目（零值表示没匹配到）。
// siteName 是站点名称；rawURL 是站点地址（外网优先），解析不出域名就跳过第三级。
func Match(siteName, rawURL string, items []Item) Item {
	if len(items) == 0 {
		return Item{}
	}
	if slug := hdicons.Slug(siteName); slug != "" {
		if item, ok := bestByBaseName(slug, items); ok {
			return item
		}
	}
	if name := strings.TrimSpace(siteName); name != "" {
		// 站点名与条目名完全相同（含纯中文条目名，如「相册」「影视」）
		if item, ok := bestByExactName(name, items); ok {
			return item
		}
		// 站点名与条目中文名相同
		if item, ok := bestByCNName(name, items); ok {
			return item
		}
	}
	if host := Host(rawURL); host != "" {
		if item, ok := bestByDomain(host, items); ok {
			return item
		}
	}
	return Item{}
}

// bestByExactName 站点名与条目 name（或去掉变体后的 name）完全相同。
// 纯中文条目名（相册 / 影视）slug 为空，走不了 slug 那条路，靠这一级兜住。
func bestByExactName(name string, items []Item) (Item, bool) {
	best := Item{}
	bestRank := -1
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Name), name) && !strings.EqualFold(item.BaseName(), name) {
			continue
		}
		rank := variantRank(item.Variant())
		if bestRank < 0 || rank < bestRank || (rank == bestRank && item.Filename < best.Filename) {
			best, bestRank = item, rank
		}
	}
	return best, bestRank >= 0
}

// bestByBaseName 站点名 slug == 条目 name（去变体）→ 命中；无变体优先，其次 A、B、C…
func bestByBaseName(slug string, items []Item) (Item, bool) {
	best := Item{}
	bestRank := -1
	for _, item := range items {
		if hdicons.Slug(item.BaseName()) != slug {
			continue
		}
		rank := variantRank(item.Variant())
		if bestRank < 0 || rank < bestRank || (rank == bestRank && item.Filename < best.Filename) {
			best, bestRank = item, rank
		}
	}
	return best, bestRank >= 0
}

// bestByCNName 站点名与 cnName 精确相等。
func bestByCNName(name string, items []Item) (Item, bool) {
	want := strings.TrimSpace(name)
	best := Item{}
	bestRank := -1
	for _, item := range items {
		cn := strings.TrimSpace(item.CNName)
		if cn == "" || !strings.EqualFold(cn, want) {
			continue
		}
		rank := variantRank(item.Variant())
		if bestRank < 0 || rank < bestRank || (rank == bestRank && item.Filename < best.Filename) {
			best, bestRank = item, rank
		}
	}
	return best, bestRank >= 0
}

// bestByDomain 条目 domain 与站点主机相同，或站点主机是它的子域（feiniu.com ← nas.feiniu.com）。
// 只认像域名的值（含点、非 IP）；更具体的（更长的）域名优先。
func bestByDomain(host string, items []Item) (Item, bool) {
	best := Item{}
	bestLen := -1
	for _, item := range items {
		domain := strings.ToLower(strings.TrimSpace(item.Domain))
		if !looksLikeDomain(domain) {
			continue
		}
		if domain != host && !strings.HasSuffix(host, "."+domain) {
			continue
		}
		rank := variantRank(item.Variant())
		better := len(domain) > bestLen
		if len(domain) == bestLen {
			bestRank := variantRank(best.Variant())
			better = best.Filename == "" || rank < bestRank || (rank == bestRank && item.Filename < best.Filename)
		}
		if better {
			best, bestLen = item, len(domain)
		}
	}
	return best, bestLen >= 0
}

// variantRank 变体优先级：无变体 0，A 1、B 2…（越小越优先）。
func variantRank(variant string) int {
	if variant == "" {
		return 0
	}
	return int(variant[0]-'A') + 1
}

// looksLikeDomain 是否像域名（含点、不是 IP 字面量）。索引里有 `qdnas-s` 这类噪声值，必须挡掉。
func looksLikeDomain(value string) bool {
	if !strings.Contains(value, ".") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	return net.ParseIP(value) == nil
}

// Host 取地址的主机名（小写、去端口、去掉开头的 www.）；解析失败或 IP 字面量返回 ""。
func Host(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}
	return strings.TrimPrefix(host, "www.")
}

// Resolve 解析手动指定的条目：filename / name / cnName 三种精确写法都接受（忽略大小写与首尾空白）。
func Resolve(items []Item, query string) Item {
	want := strings.TrimSpace(query)
	if want == "" {
		return Item{}
	}
	for _, item := range items {
		if strings.EqualFold(item.Filename, want) {
			return item
		}
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Name), want) || strings.EqualFold(item.BaseName(), want) {
			return item
		}
	}
	for _, item := range items {
		if cn := strings.TrimSpace(item.CNName); cn != "" && strings.EqualFold(cn, want) {
			return item
		}
	}
	return Item{}
}

// Suggest 搜条目（后台图标选择器用）：name / cnName 精确 → 前缀 → 包含 → filename 包含。
// 选择器里是「人看着挑」，所以这里的包含匹配是安全的（与自动匹配的保守规则无关）。
func Suggest(items []Item, query string, limit int) []string {
	want := strings.ToLower(strings.TrimSpace(query))
	if want == "" || limit <= 0 {
		return []string{}
	}
	type hit struct {
		filename string
		rank     int
	}
	hits := make([]hit, 0, limit*2)
	for _, item := range items {
		if strings.TrimSpace(item.Filename) == "" {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(item.Name))
		cn := strings.ToLower(strings.TrimSpace(item.CNName))
		filename := strings.ToLower(item.Filename)
		rank := -1
		switch {
		case name == want || cn == want:
			rank = 0
		case strings.HasPrefix(name, want) || strings.HasPrefix(cn, want):
			rank = 1
		case strings.Contains(name, want) || strings.Contains(cn, want):
			rank = 2
		case strings.Contains(filename, want):
			rank = 3
		}
		if rank >= 0 {
			hits = append(hits, hit{filename: item.Filename, rank: rank})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return hits[i].filename < hits[j].filename
	})
	out := make([]string, 0, limit)
	for _, entry := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, entry.filename)
	}
	return out
}

// ---------------------------------------------------------------- 下载

// IconURL 拼出条目图标地址；中文与特殊字符按路径规则转义（文件名里有 `+`、中文、括号）。
func IconURL(base, filename string) string {
	root := strings.TrimRight(strings.TrimSpace(base), "/")
	name := strings.TrimSpace(filename)
	if root == "" || name == "" {
		return ""
	}
	return root + (&url.URL{Path: IconDir + name}).EscapedPath()
}

// Client 索引与图标下载器；可并发使用。
type Client struct {
	mu   sync.RWMutex
	base string
	http *http.Client
}

// NewClient 创建下载器；base 为空时用 DefaultBase。
func NewClient(base string) *Client {
	client := &Client{
		http: &http.Client{
			Timeout: requestLimit,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
	client.SetBase(base)
	return client
}

// SetBase 更新站点根地址（空则回落默认）。
func (c *Client) SetBase(base string) {
	value := strings.TrimRight(strings.TrimSpace(base), "/")
	if value == "" {
		value = DefaultBase
	}
	c.mu.Lock()
	c.base = value
	c.mu.Unlock()
}

// Base 当前站点根地址。
func (c *Client) Base() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base
}

// FetchIndex 拉取索引。
func (c *Client) FetchIndex(ctx context.Context) (*Index, error) {
	root := c.Base()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/"+IndexFile, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NASVIA/0.1 (+nasicon matcher)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("索引请求返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIndexBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIndexBytes {
		return nil, errors.New("索引过大")
	}
	items, err := ParseIndex(data)
	if err != nil {
		return nil, err
	}
	return &Index{Items: items, Base: root}, nil
}

// FetchIcon 下载指定图标；会校验内容确实是图片（挡住站点返回的 HTML 错误页）。
func (c *Client) FetchIcon(ctx context.Context, filename string) ([]byte, string, error) {
	target := IconURL(c.Base(), filename)
	if target == "" {
		return nil, "", errors.New("图标地址为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "NASVIA/0.1 (+nasicon matcher)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("图标请求返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIconBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxIconBytes {
		return nil, "", errors.New("图标文件过大")
	}
	// 复用 hdicons 的魔数嗅探：一份实现、一处修改，避免两个来源的校验强度不一致。
	if !hdicons.ValidImage(data) {
		return nil, "", errors.New("返回内容不是图片")
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		contentType = contentTypeFor(filename)
	}
	return data, contentType, nil
}

// contentTypeFor 按扩展名猜类型（站点有时返回 application/octet-stream）。
func contentTypeFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".svg":
		return "image/svg+xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}

// ---------------------------------------------------------------- 索引缓存

// Index 索引 + 元信息；字段与缓存文件一致，便于直接读写。
type Index struct {
	Items     []Item    `json:"icons"`
	Base      string    `json:"-"`
	FetchedAt time.Time `json:"-"`
}

type cacheDocument struct {
	FetchedAt time.Time `json:"fetched_at"`
	Base      string    `json:"base"`
	Items     []Item    `json:"icons"`
}

// ReadCache 读索引缓存；文件缺失或损坏返回 nil。
func ReadCache(path string) *Index {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc cacheDocument
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Items) == 0 {
		return nil
	}
	return &Index{Items: doc.Items, Base: doc.Base, FetchedAt: doc.FetchedAt}
}

// WriteCache 写索引缓存（先写临时文件再原子替换）。
func WriteCache(path string, index *Index) error {
	if index == nil || len(index.Items) == 0 {
		return errors.New("索引为空")
	}
	raw, err := json.Marshal(cacheDocument{FetchedAt: time.Now(), Base: index.Base, Items: index.Items})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Fresh 缓存是否仍在有效期内（FetchedAt 为零值视为过期）。
func (i *Index) Fresh(now time.Time) bool {
	if i == nil || i.FetchedAt.IsZero() {
		return false
	}
	return now.Sub(i.FetchedAt) < IndexTTL
}
