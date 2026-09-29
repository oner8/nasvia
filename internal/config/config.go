// Package config 负责从环境变量与命令行参数加载运行配置。
package config

import (
	"flag"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oner8/nasvia/internal/hdicons"
	"github.com/oner8/nasvia/internal/nasicon"
)

// Version 构建版本，可由 -ldflags -X 覆盖。
var Version = "1.1.0"

// LegacyDefaultFaviconSources 旧版默认来源：仅在库里正好等于它时做一次性升级（加 nasicon 兜底来源）。
const LegacyDefaultFaviconSources = "hdicons,site,duckduckgo,google"

// DefaultLANRules 常用内网规则（未配置 NASVIA_LAN_CIDRS 时的默认值，与前端 DEFAULT_LAN_RULES 一致）。
// 不含 172.16.0.0/12：Docker 网桥网关也在其中，端口映射走用户态代理时会把外网直连误判为内网。
const DefaultLANRules = "192.168.0.0/16,10.0.0.0/8,*.lan,*.local,home.arpa"

// DefaultTrustedProxies 默认只信任本机与私网来的反代头（WireGuard 隧道、Docker 网桥、局域网反代都在其中）；
// 公网直连来的请求自带的 X-Forwarded-For 一律忽略，防止伪造来源 IP 绕过登录限速。
const DefaultTrustedProxies = "127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,100.64.0.0/10,fc00::/7"

// Config 运行期配置。
type Config struct {
	Bind       string
	Port       int
	DataDir    string
	DBPath     string
	AuthMode   string
	LANCIDRs   string
	HomeEgress string
	// TrustedProxies 允许其 X-Forwarded-For / X-Real-IP 生效的反代地址（CIDR / IP，逗号分隔）。
	TrustedProxies string
	SiteTitle      string
	Password       string
	FaviconSources string
	HDIconsMirrors string
	NASIconBase    string
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return fallback
}

// Load 读取配置；环境变量优先级低于显式命令行参数。
func Load(args []string) (*Config, error) {
	cfg := &Config{
		Bind:           env("NASVIA_BIND", "0.0.0.0"),
		Port:           envInt("NASVIA_PORT", 3720),
		DataDir:        env("NASVIA_DATA_DIR", "./data"),
		AuthMode:       env("NASVIA_AUTH_MODE", "public"),
		LANCIDRs:       env("NASVIA_LAN_CIDRS", DefaultLANRules),
		HomeEgress:     env("NASVIA_HOME_EGRESS", ""),
		TrustedProxies: env("NASVIA_TRUSTED_PROXIES", DefaultTrustedProxies),
		SiteTitle:      env("NASVIA_SITE_TITLE", "NASVIA"),
		Password:       os.Getenv("NASVIA_PASSWORD"),
		FaviconSources: env("NASVIA_FAVICON_SOURCES", "hdicons,nasicon,site,duckduckgo,google"),
		HDIconsMirrors: env("NASVIA_HDICONS_MIRRORS", hdicons.DefaultMirror),
		NASIconBase:    env("NASVIA_NASICON_BASE", nasicon.DefaultBase),
	}

	fs := flag.NewFlagSet("nasvia", flag.ContinueOnError)
	bind := fs.String("bind", cfg.Bind, "监听地址")
	port := fs.Int("port", cfg.Port, "监听端口")
	dataDir := fs.String("data-dir", cfg.DataDir, "数据目录")
	authMode := fs.String("auth-mode", cfg.AuthMode, "前台模式：public 或 private")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	cfg.Bind, cfg.Port, cfg.DataDir, cfg.AuthMode = *bind, *port, *dataDir, *authMode

	if v, ok := os.LookupEnv("NASVIA_DB_PATH"); ok && strings.TrimSpace(v) != "" {
		cfg.DBPath = strings.TrimSpace(v)
	} else {
		cfg.DBPath = filepath.Join(cfg.DataDir, "nasvia.db")
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)
	cfg.DBPath = filepath.Clean(cfg.DBPath)
	return cfg, nil
}

// Addr 返回 host:port 形式的监听地址。
func (c *Config) Addr() string {
	return net.JoinHostPort(c.Bind, strconv.Itoa(c.Port))
}

// IconsDir 图标缓存目录。
func (c *Config) IconsDir() string {
	return filepath.Join(c.DataDir, "icons")
}
