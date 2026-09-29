// Package netdetect 在服务端按访客 IP 判定「内网 / 外网」。
//
// 典型场景：NAS 在家里，经 WireGuard 穿透到云服务器再由反代（如 Nginx Proxy Manager）对外提供域名。
// 此时浏览器地址栏永远是同一个域名，前端按 hostname 判定失效；但反代看到的来源 IP 不同——
// 在家时是家庭宽带的公网出口 IP，在外时是别处的 IP。服务端拿「真实客户端 IP」与
//  1. 内网网段规则（直连 / 家里做了 DNS 分流时，客户端 IP 就是局域网 IP）
//  2. 家庭公网出口（静态 IP / CIDR、DDNS 域名、或 auto 自动探测）
//
// 比对，命中即判为内网。判定结果只影响站点链接用内网还是外网地址，不参与任何鉴权。
package netdetect

import (
	"net"
	"strings"
)

// 判定结果。
const (
	ModeLAN = "lan"
	ModeWAN = "wan"

	ReasonLANRule    = "lan_rule"    // 客户端 IP 命中内网网段规则
	ReasonHomeEgress = "home_egress" // 客户端 IP 等于家庭公网出口
)

// Result 服务端判定结果（随 /api/config 下发给前端）。
type Result struct {
	Mode     string `json:"mode"`
	ClientIP string `json:"client_ip"`
	Reason   string `json:"reason"`
}

// SplitList 把多行 / 逗号分隔的文本拆成条目（与前端 normalizeRules 的分隔符一致）。
func SplitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '，', '；', '\n', '\r', '\t', ' ':
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// MatchRules 用内网网段规则判定客户端 IP：只认 IP 类规则
// （CIDR、IP 前缀「10.0.」、精确 IP、以冒号区分的 IPv6 前缀），主机名类规则交给前端按地址栏判定。
func MatchRules(ip net.IP, rules []string) bool {
	if ip == nil {
		return false
	}
	text := strings.ToLower(ip.String())
	for _, raw := range rules {
		rule := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case rule == "":
			continue
		case strings.Contains(rule, "/"):
			if _, network, err := net.ParseCIDR(rule); err == nil && network.Contains(ip) {
				return true
			}
		case net.ParseIP(rule) != nil:
			if net.ParseIP(rule).Equal(ip) {
				return true
			}
		case strings.Contains(rule, ":"):
			// IPv6 前缀（fd00:、fe80:）：按文本前缀匹配，与前端一致
			if ip.To4() == nil && strings.HasPrefix(text, rule) {
				return true
			}
		case isIPv4Prefix(rule):
			if ip.To4() != nil && strings.HasPrefix(text, rule) {
				return true
			}
		}
	}
	return false
}

// isIPv4Prefix 形如「10.」「192.168.」的 IP 前缀规则。
func isIPv4Prefix(rule string) bool {
	if !strings.HasSuffix(rule, ".") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(rule, "."), ".") {
		if part == "" || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// Classify 综合判定：先看内网网段规则，再看家庭公网出口。egress 可为 nil。
func Classify(clientIP string, rules []string, egress *Egress) Result {
	res := Result{Mode: ModeWAN, ClientIP: clientIP}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return res
	}
	if MatchRules(ip, rules) {
		res.Mode, res.Reason = ModeLAN, ReasonLANRule
		return res
	}
	if egress != nil {
		if egress.Contains(ip) {
			res.Mode, res.Reason = ModeLAN, ReasonHomeEgress
			return res
		}
		// 没命中：可能是家里公网 IP 刚变过（DDNS 已更新但缓存还是旧的），异步刷新一次，下次访问即可纠正。
		egress.MaybeRefresh()
	}
	return res
}
