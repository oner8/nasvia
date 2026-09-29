// Package suggest 代理搜索引擎的联想词（OpenSearch 格式），供首页联网搜索框做下拉提示。
//
// 浏览器不能直接请求这些接口（没有 CORS 头），所以由服务端代取：
//   - 只访问内置的几个固定地址，关键词只作为查询参数，不存在 SSRF；
//   - 当前引擎取不到（例如国内 NAS 访问不了 Google）时按顺序改用其它引擎，
//     不可用的引擎短时间内直接跳过，避免每次输入都白等超时；
//   - 结果在内存里缓存一段时间，并限制同时对外的请求数。
package suggest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// MaxQueryRunes 关键词最大长度（字符）。
	MaxQueryRunes = 100
	// MaxItems 最多返回的联想词条数。
	MaxItems = 8

	attemptTimeout = 2500 * time.Millisecond
	cacheTTL       = 10 * time.Minute
	cacheCap       = 2000
	downFor        = 5 * time.Minute
	maxInFlight    = 8
)

// DefaultEndpoints 各引擎的联想接口（%s 为 URL 编码后的关键词）。
var DefaultEndpoints = map[string]string{
	"google": "https://suggestqueries.google.com/complete/search?client=firefox&ie=utf-8&oe=utf-8&hl=zh-CN&q=%s",
	"bing":   "https://api.bing.com/osjson.aspx?query=%s",
	"baidu":  "https://suggestion.baidu.com/su?action=opensearch&ie=utf-8&wd=%s",
}

// fallbackOrder 当前引擎取不到时依次尝试的引擎（国内可达的优先）。
var fallbackOrder = []string{"baidu", "bing", "google"}

type cacheEntry struct {
	items   []string
	source  string
	expires time.Time
}

// Client 联想词代理。零值不可用，请用 NewClient。
type Client struct {
	http      *http.Client
	endpoints map[string]string
	sem       chan struct{}

	mu        sync.Mutex
	cache     map[string]cacheEntry
	downUntil map[string]time.Time
}

// NewClient 使用 DefaultEndpoints 构造。
func NewClient() *Client {
	return NewClientWith(DefaultEndpoints, &http.Client{Timeout: attemptTimeout})
}

// NewClientWith 自定义接口地址与 HTTP 客户端（测试用）。
func NewClientWith(endpoints map[string]string, client *http.Client) *Client {
	return &Client{
		http:      client,
		endpoints: endpoints,
		sem:       make(chan struct{}, maxInFlight),
		cache:     map[string]cacheEntry{},
		downUntil: map[string]time.Time{},
	}
}

// order 返回本次要尝试的引擎顺序：请求的引擎在前，其余按 fallbackOrder。
func (c *Client) order(engine string) []string {
	out := make([]string, 0, len(fallbackOrder)+1)
	if _, ok := c.endpoints[engine]; ok {
		out = append(out, engine)
	}
	for _, id := range fallbackOrder {
		if _, ok := c.endpoints[id]; ok && id != engine {
			out = append(out, id)
		}
	}
	return out
}

// Suggest 返回联想词与实际提供结果的引擎；全部失败时返回空列表。
func (c *Client) Suggest(ctx context.Context, engine, query string) ([]string, string) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > MaxQueryRunes {
		return []string{}, ""
	}
	key := engine + "\x00" + strings.ToLower(query)
	now := time.Now()
	c.mu.Lock()
	if hit, ok := c.cache[key]; ok && now.Before(hit.expires) {
		c.mu.Unlock()
		return hit.items, hit.source
	}
	c.mu.Unlock()

	for _, id := range c.order(engine) {
		c.mu.Lock()
		down := now.Before(c.downUntil[id])
		c.mu.Unlock()
		if down {
			continue
		}
		items, err := c.fetch(ctx, id, query)
		if err != nil {
			if ctx.Err() != nil {
				return []string{}, "" // 请求方已取消（用户继续输入），不算引擎故障
			}
			c.mu.Lock()
			c.downUntil[id] = time.Now().Add(downFor)
			c.mu.Unlock()
			continue
		}
		c.mu.Lock()
		if len(c.cache) >= cacheCap {
			c.cache = map[string]cacheEntry{} // 简单粗暴的容量上限：满了整体清空
		}
		c.cache[key] = cacheEntry{items: items, source: id, expires: time.Now().Add(cacheTTL)}
		delete(c.downUntil, id)
		c.mu.Unlock()
		return items, id
	}
	return []string{}, ""
}

func (c *Client) fetch(ctx context.Context, engine, query string) ([]string, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	endpoint := fmt.Sprintf(c.endpoints[engine], url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (NASVIA)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	return ParseOpenSearch(body)
}

// ParseOpenSearch 解析 OpenSearch 联想格式：["关键词", ["联想1", "联想2", ...], ...]。
func ParseOpenSearch(body []byte) ([]string, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) < 2 {
		return nil, errors.New("不是 OpenSearch 联想格式")
	}
	var list []string
	if err := json.Unmarshal(raw[1], &list); err != nil {
		return nil, errors.New("不是 OpenSearch 联想格式")
	}
	out := make([]string, 0, MaxItems)
	seen := map[string]bool{}
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if len(out) == MaxItems {
			break
		}
	}
	return out, nil
}
