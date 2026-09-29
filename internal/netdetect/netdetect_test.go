package netdetect

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestMatchRules(t *testing.T) {
	rules := []string{"192.168.1.0/24", "10.0.", "172.20.1.5", "fd00:", "home.arpa", "*.lan"}
	cases := map[string]bool{
		"192.168.1.23":     true,
		"192.168.2.1":      false,
		"10.0.3.4":         true,
		"10.1.0.1":         false,
		"172.20.1.5":       true,
		"172.20.1.6":       false,
		"fd00::1234":       true,
		"2001:db8:8000::1": false,
		"203.0.113.9":      false,
	}
	for ip, want := range cases {
		if got := MatchRules(net.ParseIP(ip), rules); got != want {
			t.Errorf("MatchRules(%s) = %v, want %v", ip, got, want)
		}
	}
	if MatchRules(nil, rules) {
		t.Error("nil IP 不应命中")
	}
}

func TestParseEntries(t *testing.T) {
	entries, invalid := ParseEntries("auto, https://NAS.example.com:3720/x\n203.0.113.7；2001:db8:8000::/48 nas.example.com 不是域名 bad_host.com")
	want := []string{"auto", "nas.example.com", "203.0.113.7", "2001:db8:8000::/48"}
	if strings.Join(entries, "|") != strings.Join(want, "|") {
		t.Fatalf("entries = %v, want %v", entries, want)
	}
	if len(invalid) != 2 {
		t.Fatalf("invalid = %v, want 2 条", invalid)
	}
}

func newFakeEgress(lookup map[string][]string, detect map[string]string) *Egress {
	e := NewEgress()
	e.Lookup = func(_ context.Context, host string) ([]net.IP, error) {
		raw, ok := lookup[host]
		if !ok {
			return nil, errors.New("nxdomain")
		}
		var ips []net.IP
		for _, r := range raw {
			ips = append(ips, net.ParseIP(r))
		}
		return ips, nil
	}
	e.Detect = func(_ context.Context, network string) (net.IP, error) {
		if raw, ok := detect[network]; ok {
			return net.ParseIP(raw), nil
		}
		return nil, errors.New("unreachable")
	}
	return e
}

func TestEgressStaticAndDynamic(t *testing.T) {
	e := newFakeEgress(
		map[string][]string{"nas.example.com": {"203.0.113.7", "2001:db8:1:2::abcd", "192.168.1.2"}},
		map[string]string{"tcp4": "198.51.100.4"},
	)
	e.SetEntries("198.51.100.0/30\nnas.example.com\nauto")
	if err := e.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	cases := map[string]bool{
		"198.51.100.1":    true,  // 静态 CIDR
		"203.0.113.7":     true,  // DDNS A 记录
		"203.0.113.8":     false, // IPv4 精确匹配
		"2001:db8:1:2::1": true,  // DDNS AAAA 所在 /64
		"2001:db8:1:3::1": false,
		"198.51.100.4":    true, // auto 探测到的出口
		"192.168.1.2":     false,
		"2001:db8::1":     false,
	}
	for ip, want := range cases {
		if got := e.Contains(net.ParseIP(ip)); got != want {
			t.Errorf("Contains(%s) = %v, want %v", ip, got, want)
		}
	}
	if st := e.Status(); len(st.Resolved) != 3 || st.Error != "" {
		t.Errorf("status = %+v", st)
	}
}

func TestEgressKeepsLastGoodOnFailure(t *testing.T) {
	lookup := map[string][]string{"nas.example.com": {"203.0.113.7"}}
	e := newFakeEgress(lookup, nil)
	e.SetEntries("nas.example.com")
	if err := e.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	delete(lookup, "nas.example.com") // DNS 暂时失败
	if err := e.Refresh(context.Background()); err == nil {
		t.Fatal("应返回解析错误")
	}
	if !e.Contains(net.ParseIP("203.0.113.7")) {
		t.Error("解析失败时应保留上一次的结果")
	}
	e.SetEntries("") // 清空配置后旧结果作废
	if e.Contains(net.ParseIP("203.0.113.7")) {
		t.Error("清空配置后不应再命中")
	}
}

func TestClassify(t *testing.T) {
	e := newFakeEgress(nil, nil)
	e.SetEntries("203.0.113.7")
	rules := []string{"192.168.1.0/24", "nas.lan"}
	if r := Classify("192.168.1.5", rules, e); r.Mode != ModeLAN || r.Reason != ReasonLANRule {
		t.Errorf("局域网直连: %+v", r)
	}
	if r := Classify("203.0.113.7", rules, e); r.Mode != ModeLAN || r.Reason != ReasonHomeEgress {
		t.Errorf("家庭出口: %+v", r)
	}
	if r := Classify("198.51.100.9", rules, e); r.Mode != ModeWAN || r.Reason != "" {
		t.Errorf("外网: %+v", r)
	}
	if r := Classify("garbage", rules, nil); r.Mode != ModeWAN {
		t.Errorf("非法 IP: %+v", r)
	}
}
