package nasicon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func item(name, cn, domain, filename string) Item {
	return Item{Name: name, CNName: cn, Domain: domain, Filename: filename, URL: "/icon/" + filename}
}

func TestParseIndex(t *testing.T) {
	raw := []byte(`[{"name":"115 A","cnName":"","domain":"","filename":"115_A.png","url":"/icon/115_A.png"},
		{"name":"Baidu","cnName":"百度","domain":"baidu.com","filename":"Baidu--百度--baidu.com.png"},
		{"name":"坏条目","filename":""}]`)
	items, err := ParseIndex(raw)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(items) != 2 || items[1].CNName != "百度" || items[1].Domain != "baidu.com" {
		t.Fatalf("解析结果不符: %+v", items)
	}
	if _, err := ParseIndex([]byte(`[]`)); err == nil {
		t.Error("空索引应报错")
	}
	if _, err := ParseIndex([]byte(`{}`)); err == nil {
		t.Error("非数组应报错")
	}
	if _, err := ParseIndex([]byte(`not json`)); err == nil {
		t.Error("坏 JSON 应报错")
	}
}

func TestItemVariant(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		variant string
	}{
		{"115 A", "115", "A"},
		{"Baidunetdisk B", "Baidunetdisk", "B"},
		{"影视", "影视", ""},
		{"1Panel", "1Panel", ""}, // 末尾是大写字母但不是「空格+字母」，不算变体
		{"", "", ""},
	}
	for _, tc := range cases {
		it := item(tc.name, "", "", "x.png")
		if got := it.BaseName(); got != tc.base {
			t.Errorf("BaseName(%q) = %q，期望 %q", tc.name, got, tc.base)
		}
		if got := it.Variant(); got != tc.variant {
			t.Errorf("Variant(%q) = %q，期望 %q", tc.name, got, tc.variant)
		}
	}
}

func TestMatch(t *testing.T) {
	index := []Item{
		item("115 A", "", "", "115_A.png"),
		item("115 B", "", "", "115_B.png"),
		item("Baidunetdisk A", "百度网盘", "qdnas-s", "Baidunetdisk_A--百度网盘--qdnas-s.png"),
		item("Baidunetdisk B", "百度网盘", "qdnas-s", "Baidunetdisk_B--百度网盘--qdnas-s.png"),
		item("BaiduNetdisk", "百度网盘年卡特惠", "pan.baidu.com", "BaiduNetdisk--百度网盘年卡特惠--pan.baidu.com.png"),
		item("Baidu", "百度", "baidu.com", "Baidu--百度--baidu.com.png"),
		item("FeiniuPhoto", "飞牛相册", "feiniu.com", "FeiniuPhoto--飞牛相册--feiniu.com.png"),
		item("Jellyfin", "", "", "Jellyfin.png"),
		item("Yesplaymusic", "网易云音乐", "qdnas-s", "Yesplaymusic--网易云音乐--qdnas-s.png"),
		item("相册", "", "", "相册.png"),
		item("影视", "", "", "影视.png"),
	}

	cases := []struct {
		site    string
		url     string
		want    string
		comment string
	}{
		{site: "Jellyfin", want: "Jellyfin.png", comment: "英文名精确（无变体）"},
		{site: "115", want: "115_A.png", comment: "同一品牌的多个变体取 A"},
		{site: "百度网盘", url: "https://pan.baidu.com/", want: "Baidunetdisk_A--百度网盘--qdnas-s.png",
			comment: "中文名精确 → 变体取 A（不会被促销图百度网盘年卡特惠抢走）"},
		{site: "百度网盘年卡特惠", want: "BaiduNetdisk--百度网盘年卡特惠--pan.baidu.com.png", comment: "中文名精确"},
		{site: "某网盘服务", url: "https://pan.baidu.com/", want: "BaiduNetdisk--百度网盘年卡特惠--pan.baidu.com.png",
			comment: "域名精确命中（pan.baidu.com 比 baidu.com 更具体）"},
		{site: "某相册", url: "https://nas.feiniu.com/", want: "FeiniuPhoto--飞牛相册--feiniu.com.png",
			comment: "域名后缀命中（feiniu.com ← nas.feiniu.com）"},
		{site: "Baidu", want: "Baidu--百度--baidu.com.png", comment: "英文名精确"},
		{site: "相册", want: "相册.png", comment: "纯中文条目名精确命中（对方 name 本身就是中文）"},
		{site: "影视", want: "影视.png", comment: "同上"},
		{site: "我的相册", want: "", comment: "通用名不做包含匹配（「我的相册」不自动配到「相册」，交给手动指定）"},
		{site: "", want: "", comment: "空名不匹配"},
	}
	for _, tc := range cases {
		got := Match(tc.site, tc.url, index)
		if got.Filename != tc.want {
			t.Errorf("Match(%q, %q) = %q，期望 %q（%s）", tc.site, tc.url, got.Filename, tc.want, tc.comment)
		}
	}

	// 反向守卫：中文名只做精确，绝不做前缀/包含
	if got := Match("百度网盘", "", index[:6]); got.Filename != "Baidunetdisk_A--百度网盘--qdnas-s.png" {
		t.Errorf("「百度网盘」应精确命中百度网盘条目，得到 %q", got.Filename)
	}
	onlyPromo := []Item{item("BaiduNetdisk", "百度网盘年卡特惠", "", "promo.png")}
	if got := Match("百度网盘", "", onlyPromo); got.Filename != "" {
		t.Errorf("「百度网盘」不该被配到促销图「百度网盘年卡特惠」，却得到 %q", got.Filename)
	}
	// 域名噪声（qdnas-s 不是域名）不能参与匹配
	noisy := []Item{item("DSM6", "群晖DSM6", "qdnas-s", "DSM6--群晖DSM6--qdnas-s.png")}
	if got := Match("未知服务", "https://qdnas-s/", noisy); got.Filename != "" {
		t.Errorf("非域名的 domain 值不该命中，却得到 %q", got.Filename)
	}
	// 英文名也只做精确：不能把 File 配到 file-browser
	fuzzy := []Item{item("FileBrowser", "", "", "FileBrowser.png")}
	if got := Match("File", "", fuzzy); got.Filename != "" {
		t.Errorf("「File」不该配到 FileBrowser，却得到 %q", got.Filename)
	}
}

func TestHost(t *testing.T) {
	cases := map[string]string{
		"https://pan.baidu.com/":       "pan.baidu.com",
		"http://nas.feiniu.com:5000/x": "nas.feiniu.com",
		"www.speedtest.net":            "speedtest.net",
		"Https://Music.163.com/":       "music.163.com",
		"192.168.1.10:8096":            "",
		"http://[::1]:3720":            "",
		"://bad":                       "",
		"":                             "",
	}
	for in, want := range cases {
		if got := Host(in); got != want {
			t.Errorf("Host(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	index := []Item{
		item("Baidunetdisk A", "百度网盘", "qdnas-s", "Baidunetdisk_A--百度网盘--qdnas-s.png"),
		item("Jellyfin", "", "", "Jellyfin.png"),
	}
	cases := map[string]string{
		"Baidunetdisk_A--百度网盘--qdnas-s.png": "Baidunetdisk_A--百度网盘--qdnas-s.png", // 文件名
		"Baidunetdisk":   "Baidunetdisk_A--百度网盘--qdnas-s.png", // name 去变体
		"Baidunetdisk A": "Baidunetdisk_A--百度网盘--qdnas-s.png", // 完整 name
		"百度网盘":           "Baidunetdisk_A--百度网盘--qdnas-s.png", // 中文名
		"Jellyfin":       "Jellyfin.png",
		"不存在":            "",
		"":               "",
	}
	for in, want := range cases {
		if got := Resolve(index, in); got.Filename != want {
			t.Errorf("Resolve(%q) = %q，期望 %q", in, got.Filename, want)
		}
	}
}

func TestSuggest(t *testing.T) {
	index := []Item{
		item("Baidunetdisk A", "百度网盘", "qdnas-s", "Baidunetdisk_A--百度网盘--qdnas-s.png"),
		item("BaiduNetdisk", "百度网盘年卡特惠", "pan.baidu.com", "BaiduNetdisk--百度网盘年卡特惠--pan.baidu.com.png"),
		item("Baidu", "百度", "baidu.com", "Baidu--百度--baidu.com.png"),
		item("Jellyfin", "", "", "Jellyfin.png"),
	}
	found := Suggest(index, "百度网盘", 5)
	if len(found) == 0 || found[0] != "Baidunetdisk_A--百度网盘--qdnas-s.png" {
		t.Errorf("中文精确搜索应把精确条目排第一，得到 %v", found)
	}
	if got := Suggest(index, "jelly", 5); len(got) != 1 || got[0] != "Jellyfin.png" {
		t.Errorf("英文前缀搜索 = %v，期望 [Jellyfin.png]", got)
	}
	if got := Suggest(index, "baidu", 5); len(got) != 3 || got[0] != "Baidu--百度--baidu.com.png" {
		t.Errorf("baidu 搜索 = %v，期望 3 条且 Baidu 在前", got)
	}
	if got := Suggest(index, "百度", 2); len(got) != 2 {
		t.Errorf("limit 应生效，得到 %v", got)
	}
	if got := Suggest(index, "zzz", 5); len(got) != 0 {
		t.Errorf("无命中应为空，得到 %v", got)
	}
	if got := Suggest(index, "", 5); len(got) != 0 {
		t.Errorf("空查询应为空，得到 %v", got)
	}
	if got := Suggest(index, "jelly", 0); len(got) != 0 {
		t.Errorf("limit<=0 应为空，得到 %v", got)
	}
}

func TestIconURL(t *testing.T) {
	cases := []struct{ base, filename, want string }{
		{"https://nasicon.top", "115_A.png", "https://nasicon.top/icon/115_A.png"},
		{"https://nasicon.top/", "115_A.png", "https://nasicon.top/icon/115_A.png"},
		// 中文按路径规则转义（%E7%BD%91...），加号在路径里是合法字符、保持原样
		{"https://nasicon.top", "Baidunetdisk_A--百度网盘--qdnas-s.png",
			"https://nasicon.top/icon/Baidunetdisk_A--%E7%99%BE%E5%BA%A6%E7%BD%91%E7%9B%98--qdnas-s.png"},
		{"https://nasicon.top", "Disney+_A.png", "https://nasicon.top/icon/Disney+_A.png"},
		{"", "x.png", ""},
		{"https://nasicon.top", "", ""},
	}
	for _, tc := range cases {
		if got := IconURL(tc.base, tc.filename); got != tc.want {
			t.Errorf("IconURL(%q, %q) = %q，期望 %q", tc.base, tc.filename, got, tc.want)
		}
	}
	if !strings.Contains(IconURL("https://nasicon.top", "影视.png"), "%E5%BD%B1%E8%A7%86") {
		t.Error("纯中文文件名也应被转义")
	}
}

func TestIndexCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nasicon.json")
	if ReadCache(path) != nil {
		t.Fatal("缓存不存在时应返回 nil")
	}
	index := &Index{Items: []Item{item("Jellyfin", "", "", "Jellyfin.png")}, Base: "https://nasicon.top"}
	if err := WriteCache(path, index); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}
	loaded := ReadCache(path)
	if loaded == nil || len(loaded.Items) != 1 || loaded.Items[0].Filename != "Jellyfin.png" {
		t.Fatalf("缓存读回不符: %+v", loaded)
	}
	if loaded.Base != "https://nasicon.top" {
		t.Errorf("Base 应随缓存保留，得到 %q", loaded.Base)
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
	if err := WriteCache(path, &Index{}); err == nil {
		t.Error("空索引不应写入")
	}
	if err := os.WriteFile(path, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ReadCache(path) != nil {
		t.Error("损坏缓存应返回 nil")
	}
}
