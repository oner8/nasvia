package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/auth"
	"github.com/oner8/nasvia/internal/config"
	"github.com/oner8/nasvia/internal/favicon"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/netdetect"
)

func (s *Server) handleHealth(c *gin.Context) {
	ok(c, gin.H{"status": "ok", "version": config.Version, "time": time.Now().UTC()})
}

// configDTO 前端启动所需的全部公开配置；不含任何私密内容。
type configDTO struct {
	SiteTitle          string          `json:"site_title"`
	AuthMode           string          `json:"auth_mode"`
	LANCIDRs           []string        `json:"lan_cidrs"`
	LANCIDRsRaw        string          `json:"lan_cidrs_raw"`
	FaviconSources     favicon.Sources `json:"favicon_sources"`
	Authenticated      bool            `json:"authenticated"`
	PasswordConfigured bool            `json:"password_configured"`
	Version            string          `json:"version"`
	// NavPosition 分类导航位置：top / left / right（后台统一设置）。
	NavPosition string `json:"nav_position"`
	// NavStyle 分类导航标签样式：text（显示名称）/ icon（仅图标）。
	NavStyle string `json:"nav_style"`
	// NavVisible 分类导航是否显示：show / hide（后台统一设置）。
	NavVisible string `json:"nav_visible"`
	// Network 服务端按访客真实 IP 的内外网判定（经反代访问同一个域名时，前端只能靠它区分在家 / 在外）。
	Network netdetect.Result `json:"network"`
}

func (s *Server) configPayload(c *gin.Context) configDTO {
	return configDTO{
		SiteTitle:          s.siteTitle(),
		AuthMode:           s.authMode(),
		LANCIDRs:           s.lanCIDRs(),
		LANCIDRsRaw:        s.store.Setting(model.SettingLANCIDRs, s.cfg.LANCIDRs),
		FaviconSources:     s.fetcher.Sources(),
		Authenticated:      s.authenticated(c),
		PasswordConfigured: s.passwordConfigured(),
		Version:            config.Version,
		NavPosition:        s.navPosition(),
		NavStyle:           s.navStyle(),
		NavVisible:         s.navVisible(),
		Network:            s.network(c),
	}
}

func (s *Server) handleConfig(c *gin.Context) {
	ok(c, s.configPayload(c))
}

func (s *Server) handleSession(c *gin.Context) {
	ok(c, gin.H{
		"authenticated":       s.authenticated(c),
		"auth_mode":           s.authMode(),
		"password_configured": s.passwordConfigured(),
	})
}

type loginRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleLogin(c *gin.Context) {
	if !s.passwordConfigured() {
		fail(c, http.StatusForbidden, "password_not_set",
			"未设置密码，仅限内网访问：请设置 NASVIA_PASSWORD 环境变量后再登录")
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	key := auth.NormalizeRemote(c.ClientIP())
	// 先占名额再校验：Take 在锁内完成「检查 + 计数」，并发请求无法一起穿过限速。
	if retry := s.limiter.Take(key); retry > 0 {
		c.Header("Retry-After", strconv.Itoa(retry))
		fail(c, http.StatusTooManyRequests, "too_many_attempts",
			"登录失败次数过多，请 "+strconv.Itoa(retry)+" 秒后重试")
		return
	}
	if !s.verifyPassword(req.Password) {
		fail(c, http.StatusUnauthorized, "invalid_password", "密码不正确")
		return
	}
	s.limiter.Reset(key)

	expires, err := s.startSession(c)
	if err != nil {
		fail(c, http.StatusInternalServerError, "session_error", "创建会话失败")
		return
	}
	ok(c, gin.H{"authenticated": true, "expires_at": expires})
}

// startSession 签发新会话并写入 Cookie（登录、改密码后换发共用）。
func (s *Server) startSession(c *gin.Context) (time.Time, error) {
	token, expires, err := s.sessions.Create()
	if err != nil {
		return time.Time{}, err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
	})
	return expires, nil
}

func (s *Server) handleLogout(c *gin.Context) {
	if token, err := c.Cookie(auth.CookieName); err == nil {
		s.sessions.Revoke(token)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	ok(c, gin.H{"authenticated": false})
}

// verifyPassword 数据库散列优先，其次环境变量密码。
func (s *Server) verifyPassword(password string) bool {
	if strings.TrimSpace(password) == "" {
		return false
	}
	hash, err := s.store.PasswordHash()
	if err == nil && hash != "" {
		return auth.VerifyPassword(hash, password)
	}
	return auth.SecureEqual(s.cfg.Password, password)
}
