package hdicons

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const repo = "https://raw.githubusercontent.com/xushier/HD-Icons/main"

// items 造索 entries（默认放 border-radius 目录，与真实索引一致）。
func items(names ...string) []Item {
	out := make([]Item, 0, len(names))
	for _, name := range names {
		out = append(out, Item{Name: name, URL: repo + "/border-radius/" + name + ".png"})
	}
	return out
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Jellyfin":            "jellyfin",
		"Nginx Proxy Manager": "nginx-proxy-manager",
		"Node-RED":            "node-red",
		"ESPHome":             "esphome",
		"code-server":         "code-server",
		"群晖 DSM":              "dsm",
		"  AdGuard  Home  ":   "adguard-home",
		"Cloudflare 1.1.1.1":  "cloudflare-1-1-1-1",
		"!!!":                 "",
		"":                    "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestDomainLabels(t *testing.T) {
	cases := map[string][]string{
		"https://dsm.home.example.com:5001/webapi": {"dsm", "example"},
		"http://files.home.example.com":            {"files", "example"},
		"www.example.com":                          {"example"},
		"github.com":                               {"github"},
		"pan.baidu.com":                            {"pan", "baidu"}, // 品牌词在第二段，不能只取最左
		"http://nas.example.com:1988/":             {"nas", "example"},
		"http://[::1]:3720":                        nil, // IPv6 字面量
		"192.168.1.10":                             nil, // 纯 IP 没有品牌信息
		"://bad":                                   nil,
		"":                                         nil,
	}
	for in, want := range cases {
		got := domainLabels(in)
		if len(got) != len(want) {
			t.Errorf("domainLabels(%q) = %v，期望 %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("domainLabels(%q)[%d] = %q，期望 %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestItemPathAndRounded(t *testing.T) {
	cases := []struct {
		url     string
		path    string
		rounded bool
	}{
		{repo + "/border-radius/jellyfin-1.png", "border-radius/jellyfin-1.png", true},
		{repo + "/svg/adguard-home-1.svg", "svg/adguard-home-1.svg", false},
		{repo + "/circle/cloudflare-1.png", "circle/cloudflare-1.png", false},
		{"https://example.invalid/a/b/c.png", "b/c.png", false},
		{"", "", false},
	}
	for _, tc := range cases {
		item := Item{Name: "x", URL: tc.url}
		if got := item.Path(); got != tc.path {
			t.Errorf("Path(%q) = %q，期望 %q", tc.url, got, tc.path)
		}
		// 空 URL 的 Rounded 允许为 false；其余按目录判断
		if got := item.Rounded(); got != tc.rounded {
			t.Errorf("Rounded(%q) = %v，期望 %v", tc.url, got, tc.rounded)
		}
	}

	// AssetPath 只认圆角矩形目录，取不到返回 ""
	mixed := []Item{
		{Name: "adguard-home-1", URL: repo + "/svg/adguard-home-1.svg"},
		{Name: "adguard-1", URL: repo + "/border-radius/adguard-1.png"},
	}
	if got := AssetPath(mixed, "adguard-home-1"); got != "" {
		t.Errorf("svg 目录的条目不应被采用，得到 %q", got)
	}
	if got := AssetPath(mixed, "adguard-1"); got != "border-radius/adguard-1.png" {
		t.Errorf("AssetPath(adguard-1) = %q", got)
	}
	if got := AssetPath(mixed, "不存在"); got != "" {
		t.Errorf("不存在的条目应返回空，得到 %q", got)
	}
}

func TestParseIndex(t *testing.T) {
	raw := []byte(`{"name":"x","total_count":2,"update_at":"2026-01-01","icons":[{"name":"a-1","url":"u1"},{"name":"b-1","url":"u2"}]}`)
	idx, err := ParseIndex(raw)
	if err != nil {
		t.Fatalf("ParseIndex 失败: %v", err)
	}
	if idx.Count != 2 || len(idx.Items) != 2 || idx.Items[0].Name != "a-1" {
		t.Fatalf("解析结果不符: %+v", idx)
	}

	// 缺 total_count 时用条目数兜底
	idx, err = ParseIndex([]byte(`{"icons":[{"name":"a-1","url":"u"}]}`))
	if err != nil || idx.Count != 1 {
		t.Fatalf("缺 count 时应兜底为条目数，got %+v err=%v", idx, err)
	}

	if _, err := ParseIndex([]byte(`{"icons":[]}`)); err == nil {
		t.Error("空索引应报错")
	}
	if _, err := ParseIndex([]byte(`not json`)); err == nil {
		t.Error("坏 JSON 应报错")
	}
}

func TestMatch(t *testing.T) {
	index := items("jellyfin-1", "jellyfin-2", "immich-1", "bitwarden-1", "bitwarden-2",
		"synology-1", "synology-2", "synology-file-1", "file-browser-1", "nginx-proxy-manager-1",
		"adguard-home-1", "adguard-1", "cloudflare-1", "bare", "foo-pro", "mobile-drive-1")

	cases := []struct {
		name    string
		domain  string
		items   []Item
		want    string
		comment string
	}{
		{name: "Jellyfin", items: index, want: "jellyfin-1", comment: "slug-N 取 N 最小"},
		{name: "Nginx Proxy Manager", items: index, want: "nginx-proxy-manager-1"},
		{name: "bare", items: index, want: "bare", comment: "完全同名优先"},
		{name: "中国移动云盘", domain: "https://yun.139.com", items: index, want: "mobile-drive-1", comment: "别名 中国移动 → mobile + 关键词 云盘 → drive"},
		{name: "移动云盘", items: index, want: "mobile-drive-1", comment: "别名 移动云盘 → mobile"},
		{name: "File Station", items: index, want: "synology-file-1", comment: "别名优先于 file-* 误配"},
		{name: "群晖 DSM", items: index, want: "synology-1", comment: "中文名 → 别名 dsm → synology"},
		{name: "AdGuard Home", items: index, want: "adguard-home-1", comment: "同名条目在同一目录时 slug 优先"},
		{name: "Cloudflare 1.1.1.1", items: index, want: "cloudflare-1", comment: "带版本号的 slug 退回 cloudflare-1"},
		{name: "Foo", items: index, want: "", comment: "非数字后缀（foo-pro）不算命中，保守优先"},
		{name: "foo-pro", items: index, want: "foo-pro", comment: "完全同名依然可以命中"},
		{name: "未知服务", domain: "dsm.home.example.com", items: index, want: "synology-1", comment: "名字无果时用域名主标签"},
		{name: "未知服务", domain: "unmatched.example.com", items: index, want: ""},
		{name: "", items: index, want: ""},
		{name: "Jellyfin", items: nil, want: ""},
	}
	for _, tc := range cases {
		got := Match(tc.name, tc.domain, tc.items)
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %q，期望 %q（%s）", tc.name, tc.domain, got, tc.want, tc.comment)
		}
	}

	// 反向守卫：绝不做模糊包含匹配
	if got := Match("File", "", items("file-browser-1", "file-browser-2")); got != "" {
		t.Errorf("「File」不应误配到 file-browser-*，却得到 %q", got)
	}
	if got := Match("File Station", "", items("file-browser-1")); got != "" {
		t.Errorf("「File Station」在没有 synology-file 时不应乱配，却得到 %q", got)
	}
	// slug 太短（<2）不参与匹配
	if got := Match("a", "", items("a-1")); got != "" {
		t.Errorf("过短的名字不应匹配，却得到 %q", got)
	}

	// 真实索引形态：adguard-home-1 只落在 svg/，圆角那套叫 adguard-* → 应经别名落到 adguard-1
	adguardIndex := []Item{
		{Name: "adguard-home-1", URL: repo + "/svg/adguard-home-1.svg"},
		{Name: "adguard-1", URL: repo + "/border-radius/adguard-1.png"},
		{Name: "adguard-2", URL: repo + "/border-radius/adguard-2.png"},
	}
	if got := Match("AdGuard Home", "adguard", adguardIndex); got != "adguard-1" {
		t.Errorf("svg 里的同名条目应被跳过、经别名落到 adguard-1，却得到 %q", got)
	}
	// 只认 border-radius：同名条目落在 svg/circle 目录时不算命中
	offDir := []Item{
		{Name: "adguard-home-1", URL: repo + "/svg/adguard-home-1.svg"},
		{Name: "foo-1", URL: repo + "/circle/foo-1.png"},
	}
	if got := Match("adguard-home", "", offDir); got != "" {
		t.Errorf("svg 目录的条目不应命中，却得到 %q", got)
	}
	if got := Match("Foo", "", offDir); got != "" {
		t.Errorf("circle 目录的条目不应命中，却得到 %q", got)
	}
}

func TestParseMirrors(t *testing.T) {
	raw := "https://a.example/\n\n https://b.example ,https://a.example\n;https://c.example/"
	got := ParseMirrors(raw)
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	if len(got) != len(want) {
		t.Fatalf("ParseMirrors 条目数 %d，期望 %d：%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项 = %q，期望 %q", i, got[i], want[i])
		}
	}
	if len(ParseMirrors("   \n  ,  ")) != 0 {
		t.Error("空白输入应得到空列表")
	}
	if got := JoinMirrors(want); got != strings.Join(want, "\n") {
		t.Errorf("JoinMirrors = %q", got)
	}
}

// MergeMirrors：配置的镜像在前（改设置后立刻生效），索引里记录的镜像只作兜底。
func TestMergeMirrors(t *testing.T) {
	got := MergeMirrors([]string{"https://a.example/", "", "https://b.example"}, "https://b.example", "https://c.example")
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	if len(got) != len(want) {
		t.Fatalf("MergeMirrors = %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项 = %q，期望 %q", i, got[i], want[i])
		}
	}
	if got := MergeMirrors(nil, ""); len(got) != 0 {
		t.Errorf("全空输入应得到空列表，得到 %v", got)
	}
	if got := MergeMirrors(nil, "https://only.example"); len(got) != 1 || got[0] != "https://only.example" {
		t.Errorf("只有兜底镜像时应保留它，得到 %v", got)
	}
}
func TestIconURL(t *testing.T) {
	cases := []struct{ mirror, assetPath, want string }{
		{repo, "border-radius/jellyfin-1.png", repo + "/border-radius/jellyfin-1.png"},
		{"https://cdn.jsdelivr.net/gh/xushier/HD-Icons@main/", "border-radius/immich-1.png",
			"https://cdn.jsdelivr.net/gh/xushier/HD-Icons@main/border-radius/immich-1.png"},
		{"https://gh-proxy.com/https://raw.githubusercontent.com/xushier/HD-Icons/main",
			"border-radius/gitea-1.png",
			"https://gh-proxy.com/https://raw.githubusercontent.com/xushier/HD-Icons/main/border-radius/gitea-1.png"},
		{repo, "/border-radius/x-1.png", repo + "/border-radius/x-1.png"},
		{"", "border-radius/x-1.png", ""},
		{repo, "", ""},
	}
	for _, tc := range cases {
		if got := IconURL(tc.mirror, tc.assetPath); got != tc.want {
			t.Errorf("IconURL(%q, %q) = %q，期望 %q", tc.mirror, tc.assetPath, got, tc.want)
		}
	}
}

func TestValidImage(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	if !ValidImage(png) {
		t.Error("PNG 魔数应通过")
	}
	for _, ok := range [][]byte{
		[]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"),
		[]byte("<?xml version=\"1.0\"?><svg/>"),
		append([]byte("GIF89a"), make([]byte, 8)...),
		append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 8)...),
		append([]byte("RIFF____WEBPVP8 "), make([]byte, 4)...),
	} {
		if !ValidImage(ok) {
			t.Errorf("应识别为图片: %q", string(ok[:min(16, len(ok))]))
		}
	}
	for _, bad := range [][]byte{
		[]byte("404: Not Found"), // 镜像/代理返回的错误页（真实踩到过）
		[]byte("<!doctype html><html>404 not found</html>"),
		[]byte("short"),
		{},
	} {
		if ValidImage(bad) {
			t.Errorf("不应识别为图片: %q", string(bad))
		}
	}
}

func TestIndexCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hdicons.json")
	if ReadCache(path) != nil {
		t.Fatal("缓存不存在时应返回 nil")
	}

	idx := &Index{Name: "x", Count: 2, Items: items("a-1", "b-1"), Mirror: "https://m.example"}
	if err := WriteCache(path, idx); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}
	loaded := ReadCache(path)
	if loaded == nil || len(loaded.Items) != 2 || loaded.Mirror != "https://m.example" {
		t.Fatalf("缓存读回不符: %+v", loaded)
	}
	if !loaded.Fresh(time.Now()) {
		t.Error("刚写入的缓存应是新鲜的")
	}
	if loaded.Fresh(time.Now().Add(8 * 24 * time.Hour)) {
		t.Error("超过 TTL 后应视为过期")
	}
	if (&Index{}).Fresh(time.Now()) {
		t.Error("零值时间应视为过期")
	}

	// 损坏的缓存文件 → nil（触发重新抓取）
	if err := os.WriteFile(path, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ReadCache(path) != nil {
		t.Error("损坏缓存应返回 nil")
	}

	// 缓存文档不是索引本身，不应被 ParseIndex 误当成索引
	raw, _ := json.Marshal(cacheDocument{FetchedAt: time.Now(), Index: idx})
	if _, err := ParseIndex(raw); err == nil {
		t.Error("缓存文档不应被 ParseIndex 当成索引（保证结构变化时能被发现）")
	}
}

func TestClientMirrors(t *testing.T) {
	client := NewClient(nil)
	if got := client.Mirrors(); len(got) != 1 || got[0] != DefaultMirror {
		t.Fatalf("空配置应回落默认镜像，got %v", got)
	}

	client.SetMirrors([]string{" https://a.example/ ", "", "https://b.example"})
	got := client.Mirrors()
	if len(got) != 2 || got[0] != "https://a.example" || got[1] != "https://b.example" {
		t.Fatalf("SetMirrors 结果不符: %v", got)
	}

	// 返回副本：外部改动不影响内部状态
	got[0] = "mutated"
	if client.Mirrors()[0] != "https://a.example" {
		t.Error("Mirrors() 应返回副本")
	}

	client.SetMirrors(nil)
	if client.Mirrors()[0] != DefaultMirror {
		t.Error("清空后应回落默认镜像")
	}
}

func TestTokensAndNameTokens(t *testing.T) {
	tokens := map[string][]string{
		"baidu-drive-1": {"baidu", "drive"},
		"115-drive-1":   {"115", "drive"},
		"12306-1":       {"12306"},
		"jellyfin-1":    {"jellyfin"},
		"1panel-1":      {"1panel"},
		"foo":           {"foo"},
		"":              nil,
	}
	for in, want := range tokens {
		got := NameTokens(in)
		if len(got) != len(want) {
			t.Errorf("NameTokens(%q) = %v，期望 %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("NameTokens(%q)[%d] = %q，期望 %q", in, i, got[i], want[i])
			}
		}
	}
	// 中文一律当分隔符，只留下 ASCII token
	if got := Tokens("百度网盘 baidu-drive"); len(got) != 2 || got[0] != "baidu" || got[1] != "drive" {
		t.Errorf("Tokens(中英混排) = %v，期望 [baidu drive]", got)
	}
	if got := Tokens("群晖相册"); len(got) != 0 {
		t.Errorf("纯中文应切不出 token，得到 %v", got)
	}
}

// TestMatchChineseNames 中文名场景：中文切不出 ASCII token，靠别名表 + 中文关键词 + 域名品牌词命中。
func TestMatchChineseNames(t *testing.T) {
	index := items("baidu-1", "baidu-drive-1", "baidu-drive-2", "115-drive-1", "ali-drive-1", "ali-cloud-1",
		"synology-1", "synology-photo-1", "synology-file-1", "netease-music-1", "bilibili-1", "xunlei-1",
		"file-browser-1", "movie-pilot-1", "clouddrive2-1", "ugreen-nas-1")

	cases := []struct {
		name    string
		domain  string
		want    string
		comment string
	}{
		{name: "百度网盘", domain: "https://pan.baidu.com/", want: "baidu-drive-1", comment: "别名 → baidu-drive"},
		{name: "我的网盘", domain: "pan.baidu.com", want: "baidu-drive-1", comment: "关键词 drive + 域名品牌 baidu，多词条目胜过 baidu-1"},
		{name: "群晖相册", domain: "https://dsm.home.example.com", want: "synology-photo-1", comment: "别名 synology + 关键词 photo"},
		{name: "115网盘", domain: "https://115.com", want: "115-drive-1", comment: "别名 115-drive"},
		{name: "网易云音乐", domain: "https://music.163.com", want: "netease-music-1", comment: "别名 netease-music"},
		{name: "哔哩哔哩", domain: "https://bilibili.com", want: "bilibili-1"},
		{name: "迅雷下载", domain: "https://xunlei.com", want: "xunlei-1", comment: "域名品牌词 xunlei"},
		{name: "阿里云", domain: "aliyun.com", want: "ali-cloud-1", comment: "别名 ali + 关键词 云 → cloud"},
		{name: "影视库", domain: "movie.example.com", want: "", comment: "movie-pilot 需要 pilot，严格子集不得命中"},
		{name: "音乐库", domain: "music.163.com", want: "", comment: "只有 music 一个 token，泛词不该乱配"},
		{name: "未知服务", domain: "https://dsm.home.example.com", want: "synology-1", comment: "域名标签兜底（历史行为）"},
	}
	for _, tc := range cases {
		if got := Match(tc.name, tc.domain, index); got != tc.want {
			t.Errorf("Match(%q, %q) = %q，期望 %q（%s）", tc.name, tc.domain, got, tc.want, tc.comment)
		}
	}
}

// TestMatchCurrentLibrary 用真实索引缓存（data/hdicons.json）跑一遍现有演示站点，
// 固化「换匹配算法不改变既有结果」这条底线；缓存不存在时跳过（CI/容器里没有 data 目录）。
func TestMatchCurrentLibrary(t *testing.T) {
	index := ReadCache(filepath.Join("..", "..", "data", "hdicons.json"))
	if index == nil {
		t.Skip("没有本地索引缓存，跳过真实索引回归")
	}
	cases := []struct{ name, url, want string }{
		{"Jellyfin", "https://jellyfin.home.example.com", "jellyfin-1"},
		{"Immich", "https://immich.home.example.com", "immich-1"},
		{"Navidrome", "https://music.home.example.com", "navidrome-1"},
		{"Gitea", "https://git.home.example.com", "gitea-1"},
		{"Portainer", "https://portainer.home.example.com", "portainer-1"},
		{"code-server", "https://code.home.example.com", "code-server-1"},
		{"AdGuard Home", "https://adguard.home.example.com", "adguard-1"},
		{"Nginx Proxy Manager", "https://npm.home.example.com", "nginx-proxy-manager-1"},
		{"WireGuard", "https://vpn.home.example.com", "wireguard-1"},
		{"Home Assistant", "https://ha.home.example.com", "home-assistant-1"},
		{"ESPHome", "https://esphome.home.example.com", "esphome-1"},
		{"Node-RED", "https://nodered.home.example.com", "node-red-1"},
		{"Cloudflare 1.1.1.1", "https://one.one.one.one", "cloudflare-1"},
		{"Speedtest", "https://www.speedtest.net", "speedtest-1"},
		{"GitHub", "https://github.com", "github-1"},
		{"Vaultwarden", "https://vault.home.example.com", "bitwarden-1"},
		{"群晖 DSM", "https://dsm.home.example.com", "synology-1"},
		{"OpenWrt 路由", "https://router.home.example.com", "openwrt-1"},
		{"File Station", "https://files.home.example.com", "synology-file-1"},
		{"Moviepilot", "http://nas.example.com:1988/", ""},
		{"百度网盘 baidu-drive", "https://pan.baidu.com/", "baidu-drive-1"},
		// 新能力：中文名不再需要改名
		{"百度网盘", "https://pan.baidu.com/", "baidu-drive-1"},
		{"群晖相册", "https://dsm.home.example.com", "synology-photo-1"},
		{"115网盘", "https://115.com", "115-drive-1"},
	}
	for _, tc := range cases {
		if got := Match(tc.name, tc.url, index.Items); got != tc.want {
			t.Errorf("Match(%q, %q) = %q，期望 %q", tc.name, tc.url, got, tc.want)
		}
	}
}

func TestResolve(t *testing.T) {
	index := items("baidu-drive-1", "baidu-drive-2", "bare")
	cases := map[string]string{
		"baidu-drive-1": "baidu-drive-1", // 精确
		"baidu-drive":   "baidu-drive-1", // 前缀 + 变体号
		"baidu drive":   "baidu-drive-1", // 空格归一
		"baidu-drive-9": "",
		"不存在":           "",
		"":              "",
		"   ":           "",
	}
	for in, want := range cases {
		if got := Resolve(index, in); got != want {
			t.Errorf("Resolve(%q) = %q，期望 %q", in, got, want)
		}
	}
	// 只认 border-radius：同名条目落在 svg/ 时解析不到
	svgOnly := []Item{{Name: "adguard-home-1", URL: repo + "/svg/adguard-home-1.svg"}}
	if got := Resolve(svgOnly, "adguard-home-1"); got != "" {
		t.Errorf("svg 目录的条目不该被解析出来，得到 %q", got)
	}
}

func TestSuggest(t *testing.T) {
	index := items("baidu-1", "baidu-drive-1", "baidu-drive-2", "115-drive-1", "ali-drive-1",
		"clouddrive2-1", "synology-1", "synology-photo-1", "jellyfin-1")

	check := func(query string, limit int, want []string) {
		got := Suggest(index, query, limit)
		if len(got) != len(want) {
			t.Errorf("Suggest(%q, %d) = %v，期望 %v", query, limit, got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Suggest(%q, %d)[%d] = %q，期望 %q", query, limit, i, got[i], want[i])
			}
		}
	}
	check("baidu", 5, []string{"baidu-1", "baidu-drive-1", "baidu-drive-2"})
	check("百度", 5, []string{"baidu-1", "baidu-drive-1", "baidu-drive-2"})
	// 中文关键词 → drive：整词命中（-drive-/-drive 边界）排在 clouddrive2 这种「只是包含」的前面
	check("网盘", 5, []string{"115-drive-1", "ali-drive-1", "baidu-drive-1", "baidu-drive-2", "clouddrive2-1"})
	check("群晖", 3, []string{"synology-1", "synology-photo-1"})
	check("网盘", 2, []string{"115-drive-1", "ali-drive-1"})
	check("zzz", 5, []string{})
	check("", 5, []string{})
	check("baidu", 0, []string{})
}
