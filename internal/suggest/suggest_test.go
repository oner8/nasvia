package suggest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseOpenSearch(t *testing.T) {
	items, err := ParseOpenSearch([]byte(`["群晖",["群晖"," 群晖官网 ","群晖","","群晖nas","a","b","c","d","e","f"],[],{"x":1}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != MaxItems || items[0] != "群晖" || items[1] != "群晖官网" || items[2] != "群晖nas" {
		t.Fatalf("应去重、去空、去首尾空白并截断到 %d 条：%v", MaxItems, items)
	}
	for _, bad := range []string{`{}`, `["q"]`, `["q", "not-a-list"]`, `garbage`} {
		if _, err := ParseOpenSearch([]byte(bad)); err == nil {
			t.Errorf("%s 应解析失败", bad)
		}
	}
}

func TestSuggestFallbackAndCache(t *testing.T) {
	var googleHits, baiduHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/google", func(w http.ResponseWriter, _ *http.Request) {
		googleHits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable) // 模拟国内访问不了 Google
	})
	mux.HandleFunc("/baidu", func(w http.ResponseWriter, r *http.Request) {
		baiduHits.Add(1)
		_, _ = w.Write([]byte(`["` + r.URL.Query().Get("wd") + `",["百度联想"]]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClientWith(map[string]string{
		"google": srv.URL + "/google?q=%s",
		"baidu":  srv.URL + "/baidu?wd=%s",
	}, srv.Client())

	items, source := c.Suggest(context.Background(), "google", "nas")
	if source != "baidu" || strings.Join(items, ",") != "百度联想" {
		t.Fatalf("Google 不可用时应回退百度：%v from %q", items, source)
	}
	// 同一关键词命中缓存；另一个关键词时 Google 处于熔断期被直接跳过
	c.Suggest(context.Background(), "google", "nas")
	c.Suggest(context.Background(), "google", "docker")
	if googleHits.Load() != 1 {
		t.Fatalf("不可用的引擎在熔断期内不应再请求，实际请求 %d 次", googleHits.Load())
	}
	if baiduHits.Load() != 2 {
		t.Fatalf("相同关键词应命中缓存，百度实际请求 %d 次", baiduHits.Load())
	}

	if items, _ := c.Suggest(context.Background(), "baidu", strings.Repeat("长", MaxQueryRunes+1)); len(items) != 0 {
		t.Fatal("超长关键词应直接返回空")
	}
	if items, _ := c.Suggest(context.Background(), "baidu", "   "); len(items) != 0 {
		t.Fatal("空关键词应直接返回空")
	}
}
