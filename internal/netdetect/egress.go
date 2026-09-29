package netdetect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AutoEntry 「家庭公网出口」里的特殊条目：由 NAS 自己探测当前公网出口 IP。
const AutoEntry = "auto"

const (
	refreshTimeout = 15 * time.Second
	// minRefreshGap 按需刷新（访客判为外网时触发）的最小间隔，避免被高频请求放大成频繁的 DNS / 探测请求。
	minRefreshGap = time.Minute
	// ipv6PrefixBits 动态解析到的 IPv6 地址按 /64 比对：同一个家庭局域网内的设备共享同一个 /64 前缀。
	ipv6PrefixBits = 64
)

// 公网出口探测服务（返回纯文本 IP）：国内服务优先，国外服务兜底。按地址族强制走 IPv4 / IPv6。
var (
	detectEndpointsV4 = []string{"https://4.ipw.cn", "https://ip.3322.net", "https://api.ipify.org"}
	detectEndpointsV6 = []string{"https://6.ipw.cn", "https://api6.ipify.org"}
)

// Egress 家庭公网出口：静态 IP / CIDR 立即生效；DDNS 域名与 auto 由后台定期解析 / 探测并缓存。
type Egress struct {
	mu      sync.RWMutex
	entries []string
	static  []*net.IPNet
	// dynamic 每个动态条目（域名 / auto）最近一次成功解析的结果；某条解析失败时保留旧值。
	dynamic   map[string][]*net.IPNet
	updatedAt time.Time
	lastErr   string

	refreshing  atomic.Bool
	lastAttempt atomic.Int64

	// Lookup 解析 DDNS 域名；Detect 探测公网出口（network 为 tcp4 / tcp6）。测试可替换。
	Lookup func(ctx context.Context, host string) ([]net.IP, error)
	Detect func(ctx context.Context, network string) (net.IP, error)
}

// NewEgress 使用系统 DNS 与内置探测服务构造。
func NewEgress() *Egress {
	return &Egress{
		dynamic: map[string][]*net.IPNet{},
		Lookup: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
		Detect: detectPublicIP,
	}
}

// NormalizeEntry 归一化单个条目：auto / CIDR / IP 原样（小写）；域名去掉协议、端口与路径。
// 返回 ok=false 表示无法识别。
func NormalizeEntry(raw string) (string, bool) {
	entry := strings.ToLower(strings.TrimSpace(raw))
	if entry == "" {
		return "", false
	}
	if entry == AutoEntry {
		return entry, true
	}
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return entry, true
	}
	if net.ParseIP(entry) != nil {
		return entry, true
	}
	if i := strings.Index(entry, "://"); i >= 0 {
		entry = entry[i+3:]
	}
	if i := strings.IndexAny(entry, "/?#"); i >= 0 {
		entry = entry[:i]
	}
	if host, _, err := net.SplitHostPort(entry); err == nil {
		entry = host
	}
	entry = strings.TrimSuffix(entry, ".")
	if !validHostname(entry) {
		return "", false
	}
	return entry, true
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// ParseEntries 解析整段配置；invalid 为无法识别的原始条目（供接口返回 400）。
func ParseEntries(raw string) (entries []string, invalid []string) {
	seen := map[string]bool{}
	for _, item := range SplitList(raw) {
		entry, ok := NormalizeEntry(item)
		if !ok {
			invalid = append(invalid, item)
			continue
		}
		if !seen[entry] {
			seen[entry] = true
			entries = append(entries, entry)
		}
	}
	return entries, invalid
}

// isDynamic 需要后台解析 / 探测的条目（域名与 auto）。
func isDynamic(entry string) bool {
	if entry == AutoEntry {
		return true
	}
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return false
	}
	return net.ParseIP(entry) == nil
}

// hostNet 单个地址对应的比对范围：IPv4 精确到 /32，IPv6 放宽到所在 /64。
func hostNet(ip net.IP) *net.IPNet {
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}
	}
	mask := net.CIDRMask(ipv6PrefixBits, 128)
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

// SetEntries 替换配置：静态条目立即生效，动态条目清空旧结果并异步重新解析。
func (e *Egress) SetEntries(raw string) {
	entries, _ := ParseEntries(raw)
	var static []*net.IPNet
	for _, entry := range entries {
		if _, network, err := net.ParseCIDR(entry); err == nil {
			static = append(static, network)
		} else if ip := net.ParseIP(entry); ip != nil {
			static = append(static, hostNet(ip))
		}
	}
	e.mu.Lock()
	e.entries = entries
	e.static = static
	e.dynamic = map[string][]*net.IPNet{}
	e.updatedAt = time.Time{}
	e.lastErr = ""
	e.mu.Unlock()

	e.lastAttempt.Store(0)
	e.MaybeRefresh()
}

func (e *Egress) dynamicEntries() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []string
	for _, entry := range e.entries {
		if isDynamic(entry) {
			out = append(out, entry)
		}
	}
	return out
}

// Refresh 同步解析全部动态条目。单条失败时保留该条上一次的结果，错误汇总返回。
func (e *Egress) Refresh(ctx context.Context) error {
	entries := e.dynamicEntries()
	if len(entries) == 0 {
		return nil
	}
	e.lastAttempt.Store(time.Now().UnixNano())

	results := map[string][]*net.IPNet{}
	var errs []string
	for _, entry := range entries {
		nets, err := e.resolve(ctx, entry)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s：%v", entry, err))
			continue
		}
		results[entry] = nets
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	current := map[string]bool{}
	for _, entry := range e.entries {
		current[entry] = true
	}
	for entry, nets := range results {
		if current[entry] { // 刷新期间配置可能被改掉，丢弃已删除条目的结果
			e.dynamic[entry] = nets
		}
	}
	if len(results) > 0 {
		e.updatedAt = time.Now()
	}
	e.lastErr = strings.Join(errs, "；")
	if len(errs) > 0 {
		return errors.New(e.lastErr)
	}
	return nil
}

func (e *Egress) resolve(ctx context.Context, entry string) ([]*net.IPNet, error) {
	var ips []net.IP
	if entry == AutoEntry {
		// IPv4 / IPv6 各探测一次，任一成功即可（Docker 默认网络没有 IPv6，只拿得到 IPv4 很正常）。
		for _, network := range []string{"tcp4", "tcp6"} {
			if ip, err := e.Detect(ctx, network); err == nil && ip != nil {
				ips = append(ips, ip)
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("探测公网出口失败")
		}
	} else {
		found, err := e.Lookup(ctx, entry)
		if err != nil {
			return nil, errors.New("域名解析失败")
		}
		ips = found
	}
	var nets []*net.IPNet
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
			continue // 解析到内网地址（例如家里做了 DNS 劫持）不能代表公网出口
		}
		nets = append(nets, hostNet(ip))
	}
	if len(nets) == 0 {
		return nil, errors.New("没有得到公网地址")
	}
	return nets, nil
}

// MaybeRefresh 在有动态条目、距上次尝试超过 minRefreshGap 且没有刷新在跑时，异步刷新一次。
func (e *Egress) MaybeRefresh() {
	if len(e.dynamicEntries()) == 0 {
		return
	}
	if last := e.lastAttempt.Load(); last != 0 && time.Since(time.Unix(0, last)) < minRefreshGap {
		return
	}
	if !e.refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer e.refreshing.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()
		if err := e.Refresh(ctx); err != nil {
			log.Printf("家庭公网出口刷新失败：%v", err)
		}
	}()
}

// Run 按固定间隔刷新动态条目（阻塞，调用方用 goroutine 启动）。
func (e *Egress) Run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		e.lastAttempt.Store(0)
		e.MaybeRefresh()
	}
}

// Contains 客户端 IP 是否属于家庭公网出口。
func (e *Egress) Contains(ip net.IP) bool {
	if ip == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, network := range e.static {
		if network.Contains(ip) {
			return true
		}
	}
	for _, nets := range e.dynamic {
		for _, network := range nets {
			if network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// Status 后台展示用：动态条目当前解析到的地址、最近成功时间与错误。
type Status struct {
	Resolved  []string  `json:"resolved"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error"`
}

// Status 返回当前状态快照。
func (e *Egress) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	st := Status{UpdatedAt: e.updatedAt, Error: e.lastErr, Resolved: []string{}}
	for entry, nets := range e.dynamic {
		for _, network := range nets {
			st.Resolved = append(st.Resolved, entry+" → "+network.String())
		}
	}
	sort.Strings(st.Resolved)
	return st
}

// detectClients 按地址族强制拨号的 HTTP 客户端（tcp4 / tcp6）。
var detectClients = map[string]*http.Client{
	"tcp4": familyClient("tcp4"),
	"tcp6": familyClient("tcp6"),
}

func familyClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // 走代理会探测到代理的出口，而不是家里的
	transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second}
}

// detectPublicIP 依次请求探测服务，返回第一个合法的对应地址族的公网 IP。
func detectPublicIP(ctx context.Context, network string) (net.IP, error) {
	endpoints := detectEndpointsV4
	if network == "tcp6" {
		endpoints = detectEndpointsV6
	}
	client := detectClients[network]
	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "curl/8 (NASVIA)")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(string(body)))
		if ip == nil || (network == "tcp4") != (ip.To4() != nil) {
			continue
		}
		return ip, nil
	}
	return nil, errors.New("所有探测服务都不可用")
}
