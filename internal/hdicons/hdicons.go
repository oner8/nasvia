// Package hdicons 按站点名从 xushier/HD-Icons（MIT 许可）匹配「圆角矩形」图标并下载缓存。
//
// 仓库根目录的 icons.json 是一份 {name,url} 索引（约 1800 条）。索引里的 url 分三个目录：
// border-radius（1085 条，1024×1024 圆角矩形 PNG）、svg（矢量，圆角与直角混在一起）、circle（圆形）。
// **本包只用 border-radius 那套**，保证图标风格统一；对应条目不在该目录时继续试下一个候选名字。
//
// 匹配规则刻意保守 —— 宁可匹配不到，也不乱配：
//  1. 站点名 slug 精确等于索引里的条目名（较少见），或名称别名（Vaultwarden → bitwarden、群晖 → synology）；
//  2. `<slug>-N`（N 必须是纯数字）：取 N 最小的那个（通常就是 -1）；
//  3. 严格多词匹配（matchByTokens）：给中文名准备的 —— 中文名切不出 ASCII token，
//     只能靠中文关键词表 + 域名里的品牌词拼出 token 池，再要求候选条目的 token 全被池覆盖；
//  4. 最后的兜底才轮到域名标签（pan.baidu.com → pan / baidu）。
//
// 绝不做模糊/包含匹配：否则「File」会被 file-* 配到 File Browser、「File Station」同理。
package hdicons

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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultMirror 官方地址；国内网络常需换成镜像前缀（后台设置里可改）。
	DefaultMirror = "https://raw.githubusercontent.com/xushier/HD-Icons/main"
	// IndexFile 仓库根目录的索引文件名。
	IndexFile = "icons.json"
	// IconDir 只使用圆角矩形那套图标（索引里还有 svg / circle 两套）。
	IconDir = "border-radius"
	// IndexTTL 索引缓存有效期。
	IndexTTL = 7 * 24 * time.Hour

	maxIndexBytes = 4 << 20
	maxIconBytes  = 2 << 20
	requestLimit  = 12 * time.Second
)

// Item 索引里的一条图标。
type Item struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Path 图标在仓库里的相对路径，例如 border-radius/jellyfin-1.png。
func (i Item) Path() string {
	parsed, err := url.Parse(strings.TrimSpace(i.URL))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return ""
	}
	return strings.Join(segments[len(segments)-2:], "/")
}

// Rounded 是否是我们要的圆角矩形那套（border-radius 目录）。
func (i Item) Rounded() bool {
	return strings.HasPrefix(i.Path(), IconDir+"/")
}

// AssetPath 取某个索引条目在仓库里的相对路径；只认圆角矩形那套，找不到返回 ""。
func AssetPath(items []Item, name string) string {
	for _, item := range items {
		if item.Name == name && item.Rounded() {
			return item.Path()
		}
	}
	return ""
}

// Index 索引文档；字段与仓库里的 icons.json 保持一致，便于直接缓存回写。
type Index struct {
	Name      string    `json:"name"`
	Count     int       `json:"total_count"`
	UpdatedAt string    `json:"update_at"`
	Items     []Item    `json:"icons"`
	Mirror    string    `json:"-"`
	FetchedAt time.Time `json:"-"`
}

// ParseIndex 解析索引 JSON。
func ParseIndex(data []byte) (*Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("解析图标索引失败: %w", err)
	}
	if len(idx.Items) == 0 {
		return nil, errors.New("图标索引为空")
	}
	if idx.Count == 0 {
		idx.Count = len(idx.Items)
	}
	return &idx, nil
}

// ---------------------------------------------------------------- 镜像前缀

// ParseMirrors 解析镜像前缀：按行或逗号/分号分隔，去空白、去尾斜杠、去重。
func ParseMirrors(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		mirror := NormalizeMirror(field)
		if mirror == "" {
			continue
		}
		if _, dup := seen[mirror]; dup {
			continue
		}
		seen[mirror] = struct{}{}
		out = append(out, mirror)
	}
	return out
}

// MergeMirrors 合并镜像前缀：配置的在前、兜底的在后，去重并跳过空值。
// 用于「按当前设置下载、失败再用索引里记录的那个镜像兜底」。
func MergeMirrors(configured []string, extra ...string) []string {
	out := make([]string, 0, len(configured)+len(extra))
	seen := map[string]struct{}{}
	for _, list := range [][]string{configured, extra} {
		for _, raw := range list {
			mirror := NormalizeMirror(raw)
			if mirror == "" {
				continue
			}
			if _, dup := seen[mirror]; dup {
				continue
			}
			seen[mirror] = struct{}{}
			out = append(out, mirror)
		}
	}
	return out
}

// NormalizeMirror 去掉首尾空白与尾部斜杠。
func NormalizeMirror(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// JoinMirrors 把镜像列表序列化成多行文本（存库/回显用）。
func JoinMirrors(list []string) string {
	return strings.Join(list, "\n")
}

// IconURL 拼出某个图标（仓库相对路径）在指定镜像前缀下的地址。
// 路径来自索引条目本身（见 Item.Path），因为少数条目在 svg/ 或 circle/ 目录下。
func IconURL(mirror, assetPath string) string {
	base := NormalizeMirror(mirror)
	if base == "" || assetPath == "" {
		return ""
	}
	return base + "/" + strings.TrimPrefix(assetPath, "/")
}

// ---------------------------------------------------------------- 名称匹配

// aliases 索引里没有同名 slug 时的常见等价名。
// key 用小写原始名片段（可含中文），value 是 HD-Icons 里的 slug 前缀；长 key 优先，
// 所以「百度网盘」不会被更短的「百度」抢走。中文名切不出 ASCII token，这张表是它们最直接的线索。
var aliases = map[string]string{
	"vaultwarden":  "bitwarden",     // Vaultwarden 没有独立图标，用同源项目 Bitwarden
	"群晖":           "synology",      // 中文名 → 官方 slug
	"dsm":          "synology",      // 群晖 DSM
	"synology dsm": "synology",      // 名字里已经写了 synology，避免落空
	"file station": "synology-file", // 群晖文件站
	"中国移动":         "mobile",        // 中国移动云盘（索引里 mobile-drive-1）
	"移动云盘":         "mobile",        // 同上，名字里带「移动云盘」时也能命中
	"files":        "synology-file", // 域名 files.xxx 的常见写法
	"群晖文件站":        "synology-file",
	"adguard home": "adguard",    // 索引里 adguard-home-1 只在 svg/，圆角那套叫 adguard-1
	"cloudflare":   "cloudflare", //「Cloudflare 1.1.1.1」的 slug 会带版本号，退回 cloudflare-1
	// 常见中文品牌名（索引里只有英文名，这里是人工策展的有界表，漏了加一行即可）
	"百度网盘":        "baidu-drive",
	"115网盘":       "115-drive",
	"阿里云盘":        "ali-drive",
	"网易云音乐":       "netease-music",
	"哔哩哔哩":        "bilibili",
	"蓝奏云":         "lanzouyun",
	"aliyundrive": "ali-drive",
	"百度":          "baidu",
	"夸克":          "quark",
	"迅雷":          "xunlei",
	"阿里":          "ali",
	"zigbee2mqtt": "zigbee-mqtt", // 索引里叫 zigbee-mqtt-1，容器名 zigbee2mqtt 也能命中
}

// keywords 中文服务类型 → 索引里真实存在的英文 token（matchByTokens 用它给中文名补线索）。
// 只收索引里确实出现过的词（drive 12 条、photo 12 条、music 6 条、cloud 13 条… 都核对过）；
// 索引里 0 命中的映射（如「备份→backup」「面板→panel」）一概不写，免得白配。
var keywords = map[string]string{
	"网盘": "drive", "云盘": "drive",
	"相册": "photo", "图库": "photo", "照片": "photo",
	"音乐": "music", "影视": "video", "视频": "video", "电影": "movie",
	"阅读": "reader", "电子书": "reader", "笔记": "note",
	"监控": "cam", "摄像头": "cam", "路由": "router", "路由器": "router",
	"同步": "sync", "媒体": "media", "电视": "tv",
	"文档": "docs", "办公": "office", "搜索": "search", "云": "cloud",
}

// sortedKeys 取 map 的 key，按「长优先、同长字典序」排序，供别名/关键词表做最长匹配。
func sortedKeys(table map[string]string) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// keywordFor 按「长 key 优先」找中文关键词对应的英文 token；找不到返回 ""。
func keywordFor(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, key := range sortedKeys(keywords) {
		if strings.Contains(lower, key) {
			return keywords[key]
		}
	}
	return ""
}

// Slug 把名字规整成索引里的 slug 形态：小写、非字母数字折叠为单个 '-'、去首尾。
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// domainLabels 取域名里两段有意义的标签：最左一段（跳过 www）与顶级域名前的一段。
// pan.baidu.com → [pan baidu]：品牌词常常在第二段，只取最左那段会漏掉 baidu。
// 纯 IP 字面量没有品牌信息，返回空。
func domainLabels(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if net.ParseIP(host) != nil {
		return nil
	}
	parts := strings.Split(host, ".")
	first := parts[0]
	if first == "www" && len(parts) > 1 {
		first = parts[1]
	}
	labels := []string{first}
	if len(parts) > 1 {
		labels = append(labels, parts[len(parts)-2])
	}
	out := make([]string, 0, 2)
	for _, label := range labels {
		slug := Slug(label)
		if slug == "" {
			continue
		}
		duplicate := false
		for _, exist := range out {
			if exist == slug {
				duplicate = true
			}
		}
		if !duplicate {
			out = append(out, slug)
		}
	}
	return out
}

// Tokens 把字符串切成小写 token（非字母数字一律当分隔符，中文因此被丢掉）。
func Tokens(value string) []string {
	return strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
}

// NameTokens 索引条目名 → token，并丢掉末尾的变体号：baidu-drive-1 → [baidu drive]。
func NameTokens(indexName string) []string {
	tokens := Tokens(indexName)
	if len(tokens) > 1 {
		if _, err := strconv.Atoi(tokens[len(tokens)-1]); err == nil {
			tokens = tokens[:len(tokens)-1]
		}
	}
	return tokens
}

// aliasFor 先用原始名（小写）做子串匹配（可含中文），再用 slug 精确查表。
func aliasFor(name, slug string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, key := range sortedKeys(aliases) {
		if strings.Contains(lower, key) {
			return aliases[key]
		}
	}
	if alias, ok := aliases[slug]; ok {
		return alias
	}
	return ""
}

// roundedNames 收集 border-radius 目录下的条目名，供 pickByPrefix 判「完全同名」。
func roundedNames(items []Item) map[string]struct{} {
	names := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Rounded() {
			names[item.Name] = struct{}{}
		}
	}
	return names
}

// Match 返回匹配到的索引条目名（例如 "jellyfin-1"）；匹配不到返回 ""。只会命中 border-radius 目录下的条目。
// domain 可以是完整地址（https://pan.baidu.com/）也可以只是域名，内部自己取标签。
//
// 三段式：
//  1. 站点名 slug / 名称别名 → 「完全同名」或 `<前缀>-N`（N 必须纯数字，N 最小优先）；
//  2. 严格多词匹配（见 matchByTokens）—— 中文名切不出 ASCII token，靠中文关键词表 + 域名里的品牌词拼池；
//  3. 域名标签 / 域名别名 → 同 1。
//
// 绝不做模糊/包含匹配：否则「File」会被 file-* 配到 File Browser、「File Station」同理。
func Match(name, domain string, items []Item) string {
	if len(items) == 0 {
		return ""
	}
	names := roundedNames(items)

	candidates := make([]string, 0, 6)
	add := func(candidate string) {
		if len(candidate) < 2 {
			return
		}
		for _, exist := range candidates {
			if exist == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	slug := Slug(name)
	if slug != "" {
		add(slug)
		if alias := aliasFor(name, slug); alias != "" {
			add(alias)
		}
	}
	for _, candidate := range candidates {
		if picked := pickByPrefix(candidate, items, names); picked != "" {
			return picked
		}
	}

	if picked := matchByTokens(name, domain, items); picked != "" {
		return picked
	}
	for _, label := range domainLabels(domain) {
		if label == slug {
			continue
		}
		if picked := pickByPrefix(label, items, names); picked != "" {
			return picked
		}
		if alias := aliasFor(label, label); alias != "" {
			if picked := pickByPrefix(alias, items, names); picked != "" {
				return picked
			}
		}
	}
	return ""
}

// matchByTokens 严格多词匹配：候选条目的 token 必须「全部」出现在池里（子集关系），
// 所以光靠域名里的一个泛词（file / home / cloud）永远不会把 file-browser 之类配进来。
// 池来自：站点名 token、名称别名展开、中文关键词映射、域名标签（含其别名）。
// 命中多个时：token 多者优先（baidu-drive-1 胜过 baidu-1），再取变体号小者，最后字典序。
func matchByTokens(name, domain string, items []Item) string {
	pool := make(map[string]struct{})
	collect := func(values ...string) {
		for _, value := range values {
			for _, token := range Tokens(value) {
				pool[token] = struct{}{}
			}
		}
	}
	collect(name)
	if alias := aliasFor(name, Slug(name)); alias != "" {
		collect(alias)
	}
	if keyword := keywordFor(name); keyword != "" {
		collect(keyword)
	}
	labels := domainLabels(domain)
	collect(labels...)
	for _, label := range labels {
		if alias := aliasFor(label, label); alias != "" {
			collect(alias)
		}
	}
	// 池里至少得有一个长度 ≥2 的 token：单字符名字（a、1）是噪声，与 A1 段「slug 至少 2 字符」保持一致。
	long := false
	for token := range pool {
		if len(token) >= 2 {
			long = true
			break
		}
	}
	if !long {
		return ""
	}

	best := ""
	bestTokens := -1
	bestNumber := 0
	for _, item := range items {
		if !item.Rounded() {
			continue
		}
		tokens := NameTokens(item.Name)
		if len(tokens) == 0 {
			continue
		}
		covered := true
		for _, token := range tokens {
			if _, ok := pool[token]; !ok {
				covered = false
				break
			}
		}
		if !covered {
			continue
		}
		number := variantNumber(item.Name)
		if len(tokens) > bestTokens ||
			(len(tokens) == bestTokens && (best == "" || number < bestNumber || (number == bestNumber && item.Name < best))) {
			best, bestTokens, bestNumber = item.Name, len(tokens), number
		}
	}
	return best
}

// noVariant 用于「条目名没有变体号」：给一个很大的数，让带 -N 的条目优先。
const noVariant = 1 << 30

// variantNumber 取条目名末尾的变体号（baidu-drive-2 → 2）。
func variantNumber(indexName string) int {
	if index := strings.LastIndex(indexName, "-"); index >= 0 {
		if number, err := strconv.Atoi(indexName[index+1:]); err == nil {
			return number
		}
	}
	return noVariant
}

// Resolve 解析手动指定的图标名：精确同名优先，其次按 `<名字>-数字` 前缀匹配，
// 所以后台填 baidu-drive 也能命中 baidu-drive-1；都解析不到返回 ""。
func Resolve(items []Item, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	if AssetPath(items, query) != "" {
		return query
	}
	return pickByPrefix(Slug(query), items, roundedNames(items))
}

// Suggest 在索引里搜条目名（后台图标选择器用）：全名 → 起始 → 包含 → 英文线索命中，依次降级。
// 中文查询先经关键词表/别名表翻成英文线索（输「网盘」能列出 drive 类），只收 border-radius 那套。
func Suggest(items []Item, query string, limit int) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit <= 0 {
		return []string{}
	}
	probes := []string{query}
	if tokens := Tokens(query); len(tokens) == 0 {
		// 纯中文：翻成英文线索（关键词优先，别名补充）
		if keyword := keywordFor(query); keyword != "" {
			probes = append(probes, keyword)
		}
		if alias := aliasFor(query, ""); alias != "" {
			probes = append(probes, alias)
		}
	} else {
		probes = append(probes, tokens...)
	}

	type hit struct {
		name string
		rank int
	}
	hits := make([]hit, 0, limit*2)
	for _, item := range items {
		if !item.Rounded() {
			continue
		}
		name := strings.ToLower(item.Name)
		rank := -1
		switch {
		case name == query:
			rank = 0
		case strings.HasPrefix(name, query):
			rank = 1
		case strings.Contains(name, query):
			rank = 2
		default:
			for _, probe := range probes[1:] {
				if probe == "" {
					continue
				}
				if found := probeRank(name, probe); found >= 0 && (rank < 0 || found < rank) {
					rank = found
				}
			}
		}
		if rank >= 0 {
			hits = append(hits, hit{name: item.Name, rank: rank})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		number, other := variantNumber(hits[i].name), variantNumber(hits[j].name)
		if number != other {
			return number < other
		}
		return hits[i].name < hits[j].name
	})

	out := make([]string, 0, limit)
	for _, entry := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, entry.name)
	}
	return out
}

// probeRank 条目名对英文线索的命中档位：3 = 整词命中（-drive 边界），4 = 只是包含，-1 = 没命中。
func probeRank(name, probe string) int {
	switch {
	case name == probe, strings.HasPrefix(name, probe+"-"),
		strings.HasSuffix(name, "-"+probe), strings.Contains(name, "-"+probe+"-"):
		return 3
	case strings.Contains(name, probe):
		return 4
	default:
		return -1
	}
}

// pickByPrefix 只接受「完全同名」或「`<prefix>-N`（N 必须是纯数字）」两类，N 最小的优先；
// 且只认 border-radius 目录下的条目。非数字后缀一律不算命中：`file-*` 不能把「File」配到 file-browser。
func pickByPrefix(prefix string, items []Item, names map[string]struct{}) string {
	if _, ok := names[prefix]; ok {
		return prefix
	}
	prefixDash := prefix + "-"
	best := ""
	bestNum := 0
	for _, item := range items {
		if !item.Rounded() || !strings.HasPrefix(item.Name, prefixDash) {
			continue
		}
		num, err := strconv.Atoi(strings.TrimPrefix(item.Name, prefixDash))
		if err != nil {
			continue
		}
		if best == "" || num < bestNum || (num == bestNum && item.Name < best) {
			best, bestNum = item.Name, num
		}
	}
	return best
}

// ---------------------------------------------------------------- 下载

// Client 图标索引与图标的下载器；可并发使用。
type Client struct {
	mu      sync.RWMutex
	mirrors []string
	http    *http.Client
}

// NewClient 创建下载器；mirrors 为空时使用 DefaultMirror。
func NewClient(mirrors []string) *Client {
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
	client.SetMirrors(mirrors)
	return client
}

// SetMirrors 更新镜像前缀列表（空则回落默认）。
func (c *Client) SetMirrors(mirrors []string) {
	list := make([]string, 0, len(mirrors))
	for _, mirror := range mirrors {
		if normalized := NormalizeMirror(mirror); normalized != "" {
			list = append(list, normalized)
		}
	}
	if len(list) == 0 {
		list = []string{DefaultMirror}
	}
	c.mu.Lock()
	c.mirrors = list
	c.mu.Unlock()
}

// Mirrors 当前镜像前缀列表（副本）。
func (c *Client) Mirrors() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.mirrors...)
}

// FetchIndexFrom 从指定镜像前缀拉取索引。
func (c *Client) FetchIndexFrom(ctx context.Context, mirror string) (*Index, error) {
	base := NormalizeMirror(mirror)
	if base == "" {
		return nil, errors.New("镜像前缀为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+IndexFile, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NASVIA/0.1 (+hd-icons matcher)")
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
	index, err := ParseIndex(data)
	if err != nil {
		return nil, err
	}
	index.Mirror = base
	return index, nil
}

// FetchIndex 按配置顺序尝试各镜像前缀，返回首个成功的索引。
func (c *Client) FetchIndex(ctx context.Context) (*Index, error) {
	var lastErr error
	for _, mirror := range c.Mirrors() {
		index, err := c.FetchIndexFrom(ctx, mirror)
		if err == nil {
			return index, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用的镜像前缀")
	}
	return nil, lastErr
}

// FetchIcon 下载指定图标（仓库相对路径，见 Item.Path）；会校验内容确实是图片。
func (c *Client) FetchIcon(ctx context.Context, mirror, assetPath string) ([]byte, string, error) {
	target := IconURL(mirror, assetPath)
	if target == "" {
		return nil, "", errors.New("图标地址为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "NASVIA/0.1 (+hd-icons matcher)")
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
	if !ValidImage(data) {
		return nil, "", errors.New("返回内容不是图片")
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/png"
	}
	return data, contentType, nil
}

// ValidImage 校验下载内容确实是图片（PNG/SVG/GIF/JPEG/WebP），防止镜像返回 HTML 错误页。
func ValidImage(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	switch {
	case strings.HasPrefix(string(data), "\x89PNG\r\n\x1a\n"):
		return true
	case strings.HasPrefix(string(data), "<svg"), strings.HasPrefix(string(data), "<?xml"):
		return true
	case strings.HasPrefix(string(data), "GIF8"):
		return true
	case data[0] == 0xFF && data[1] == 0xD8:
		return true
	case strings.HasPrefix(string(data), "RIFF") && strings.Contains(string(data[:16]), "WEBP"):
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------- 索引缓存

type cacheDocument struct {
	FetchedAt time.Time `json:"fetched_at"`
	Mirror    string    `json:"mirror"`
	Index     *Index    `json:"index"`
}

// ReadCache 读取索引缓存；文件缺失或损坏时返回 nil。
func ReadCache(path string) *Index {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc cacheDocument
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Index == nil || len(doc.Index.Items) == 0 {
		return nil
	}
	doc.Index.FetchedAt = doc.FetchedAt
	if doc.Mirror != "" {
		doc.Index.Mirror = doc.Mirror
	}
	return doc.Index
}

// WriteCache 写入索引缓存（先写临时文件再原子替换）。
func WriteCache(path string, index *Index) error {
	if index == nil {
		return errors.New("索引为空")
	}
	doc := cacheDocument{FetchedAt: time.Now(), Mirror: index.Mirror, Index: index}
	raw, err := json.Marshal(doc)
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
