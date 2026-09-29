package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/auth"
	"github.com/oner8/nasvia/internal/favicon"
	"github.com/oner8/nasvia/internal/hdicons"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/nasicon"
	"github.com/oner8/nasvia/internal/netdetect"
)

type settingsDTO struct {
	AuthMode string `json:"auth_mode"`
	LANCIDRs string `json:"lan_cidrs_raw"`
	// HomeEgress 家庭公网出口原文；HomeEgressStatus 其中 DDNS 域名 / auto 当前解析到的地址。
	HomeEgress       string           `json:"home_egress_raw"`
	HomeEgressStatus netdetect.Status `json:"home_egress_status"`
	SiteTitle        string           `json:"site_title"`
	FaviconSources   favicon.Sources  `json:"favicon_sources"`
	HDIconsMirrors   string           `json:"hdicons_mirrors_raw"`
	NASIconBase      string           `json:"nasicon_base_raw"`
	// NavPosition 分类导航位置：left / right（顶部导航已取消）。
	NavPosition string `json:"nav_position"`
	// NavStyle 分类导航标签样式：text（显示名称）/ icon（仅图标）。
	NavStyle string `json:"nav_style"`
	// NavVisible 分类导航是否显示：show / hide。
	NavVisible         string `json:"nav_visible"`
	PasswordConfigured bool   `json:"password_configured"`
}

func (s *Server) settingsPayload() settingsDTO {
	return settingsDTO{
		AuthMode:           s.authMode(),
		LANCIDRs:           s.store.Setting(model.SettingLANCIDRs, s.cfg.LANCIDRs),
		HomeEgress:         s.homeEgressRaw(),
		HomeEgressStatus:   s.egress.Status(),
		SiteTitle:          s.siteTitle(),
		FaviconSources:     s.fetcher.Sources(),
		HDIconsMirrors:     s.hdMirrorsRaw(),
		NASIconBase:        s.nas.Base(),
		NavPosition:        s.navPosition(),
		NavStyle:           s.navStyle(),
		NavVisible:         s.navVisible(),
		PasswordConfigured: s.passwordConfigured(),
	}
}

func (s *Server) handleGetSettings(c *gin.Context) {
	ok(c, s.settingsPayload())
}

type settingsInput struct {
	AuthMode          *string          `json:"auth_mode"`
	LANCIDRs          *string          `json:"lan_cidrs"`
	HomeEgress        *string          `json:"home_egress"`
	SiteTitle         *string          `json:"site_title"`
	FaviconSources    *favicon.Sources `json:"favicon_sources"`
	FaviconSourcesRaw *string          `json:"favicon_sources_raw"`
	HDIconsMirrors    *string          `json:"hdicons_mirrors"`
	NavPosition       *string          `json:"nav_position"`
	NavStyle          *string          `json:"nav_style"`
	NavVisible        *string          `json:"nav_visible"`
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	var in settingsInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	fields := map[string]string{}
	nextSources := s.fetcher.Sources()

	if in.AuthMode != nil {
		mode := model.NormalizeAuthMode(*in.AuthMode)
		if strings.TrimSpace(*in.AuthMode) != "" && *in.AuthMode != model.AuthModePublic && *in.AuthMode != model.AuthModePrivate {
			fail(c, http.StatusBadRequest, "invalid_auth_mode", "auth_mode 只能是 public 或 private")
			return
		}
		if mode == model.AuthModePrivate && !s.passwordConfigured() {
			fail(c, http.StatusBadRequest, "password_required",
				"切换到「私密」模式前请先设置后台密码，否则将无人能进入站点")
			return
		}
		fields[model.SettingAuthMode] = mode
	}
	if in.LANCIDRs != nil {
		fields[model.SettingLANCIDRs] = strings.TrimSpace(*in.LANCIDRs)
	}
	if in.HomeEgress != nil {
		entries, invalid := netdetect.ParseEntries(*in.HomeEgress)
		if len(invalid) > 0 {
			fail(c, http.StatusBadRequest, "invalid_home_egress",
				"家庭公网出口无法识别："+strings.Join(invalid, "、")+"（支持 auto、IP、CIDR、DDNS 域名）")
			return
		}
		fields[model.SettingHomeEgress] = strings.Join(entries, "\n")
	}
	if in.SiteTitle != nil {
		title := strings.TrimSpace(*in.SiteTitle)
		if title == "" {
			title = "NASVIA"
		}
		fields[model.SettingSiteTitle] = title
	}
	if in.FaviconSources != nil {
		nextSources = *in.FaviconSources
		fields[model.SettingFaviconSources] = nextSources.String()
	}
	if in.FaviconSourcesRaw != nil {
		nextSources = favicon.ParseSources(*in.FaviconSourcesRaw)
		fields[model.SettingFaviconSources] = nextSources.String()
	}
	if in.HDIconsMirrors != nil {
		fields[model.SettingHDIconsMirrors] = strings.TrimSpace(*in.HDIconsMirrors)
	}
	if in.NavPosition != nil {
		value := strings.TrimSpace(*in.NavPosition)
		if value != "" && !model.ValidNavPosition(value) {
			fail(c, http.StatusBadRequest, "invalid_nav_position", "nav_position 只能是 left 或 right（顶部导航已取消）")
			return
		}
		fields[model.SettingNavPosition] = model.NormalizeNavPosition(value)
	}
	if in.NavStyle != nil {
		value := strings.TrimSpace(*in.NavStyle)
		if value != "" && !model.ValidNavStyle(value) {
			fail(c, http.StatusBadRequest, "invalid_nav_style", "nav_style 只能是 text 或 icon")
			return
		}
		fields[model.SettingNavStyle] = model.NormalizeNavStyle(value)
	}
	if in.NavVisible != nil {
		value := strings.TrimSpace(*in.NavVisible)
		if value != "" && !model.ValidNavVisible(value) {
			fail(c, http.StatusBadRequest, "invalid_nav_visible", "nav_visible 只能是 show 或 hide")
			return
		}
		fields[model.SettingNavVisible] = model.NormalizeNavVisible(value)
	}

	if err := s.store.SetSettings(fields); err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "保存配置失败")
		return
	}
	if _, ok := fields[model.SettingFaviconSources]; ok {
		s.fetcher.SetSources(nextSources)
	}
	if _, ok := fields[model.SettingHDIconsMirrors]; ok {
		s.hd.SetMirrors(hdicons.ParseMirrors(s.hdMirrorsRaw()))
		s.resetIndexBackoff()
	}
	if _, ok := fields[model.SettingHomeEgress]; ok {
		s.egress.SetEntries(s.homeEgressRaw())
	}
	ok(c, s.settingsPayload())
}

// hdMirrorsRaw 返回「生效的」HD-Icons 镜像前缀文本（设置 → 环境变量 → 内置默认）。
func (s *Server) hdMirrorsRaw() string {
	raw := strings.TrimSpace(s.store.Setting(model.SettingHDIconsMirrors, s.cfg.HDIconsMirrors))
	if raw == "" {
		raw = hdicons.DefaultMirror
	}
	return raw
}

// navPosition 返回生效的导航位置（设置缺失时回落 left；历史值 top 也会归一成 left）。
func (s *Server) navPosition() string {
	return model.NormalizeNavPosition(s.store.Setting(model.SettingNavPosition, model.NavPositionLeft))
}

// navVisible 返回「分类导航是否显示」（设置缺失时回落 show）。
func (s *Server) navVisible() string {
	return model.NormalizeNavVisible(s.store.Setting(model.SettingNavVisible, model.NavVisibleShow))
}

// navStyle 返回生效的导航标签样式（设置缺失时回落 icon，即默认仅图标）。
func (s *Server) navStyle() string {
	return model.NormalizeNavStyle(s.store.Setting(model.SettingNavStyle, model.NavStyleIcon))
}

type hdMirrorResult struct {
	Mirror string `json:"mirror"`
	OK     bool   `json:"ok"`
	MS     int64  `json:"ms"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

// handleTestHDIcons 逐条测试镜像前缀是否可用（后台「测试连通性」按钮用）。
func (s *Server) handleTestHDIcons(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	mirrors := hdicons.ParseMirrors(s.hdMirrorsRaw())
	if len(mirrors) == 0 {
		mirrors = []string{hdicons.DefaultMirror}
	}
	results := make([]hdMirrorResult, 0, len(mirrors))
	for _, mirror := range mirrors {
		start := time.Now()
		index, err := s.hd.FetchIndexFrom(ctx, mirror)
		row := hdMirrorResult{Mirror: mirror, MS: time.Since(start).Milliseconds()}
		if err != nil {
			row.Error = err.Error()
		} else {
			row.OK = true
			row.Count = index.Count
		}
		results = append(results, row)
	}

	payload := gin.H{"mirrors": results, "mirrors_raw": s.hdMirrorsRaw()}
	if cached := s.hdIndexCached(); cached != nil {
		payload["cache"] = gin.H{
			"count":      cached.Count,
			"mirror":     cached.Mirror,
			"fetched_at": cached.FetchedAt,
			"fresh":      cached.Fresh(time.Now()),
		}
	}
	ok(c, payload)
}

type passwordInput struct {
	Password string `json:"password"`
}

func (s *Server) handleChangePassword(c *gin.Context) {
	var in passwordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	password := strings.TrimSpace(in.Password)
	if len([]rune(password)) < 6 {
		fail(c, http.StatusBadRequest, "weak_password", "密码至少 6 位")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "hash_error", "生成密码散列失败")
		return
	}
	if err := s.store.SetPasswordHash(hash); err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "保存密码失败")
		return
	}
	// 所有设备（含可能被盗用的旧 Cookie）立即下线，再给当前浏览器换发新会话。
	s.sessions.RevokeAll()
	if _, err := s.startSession(c); err != nil {
		fail(c, http.StatusInternalServerError, "session_error", "密码已更新，但创建会话失败，请重新登录")
		return
	}
	ok(c, gin.H{"updated": true})
}

// handlePurgeAll 一键清空站点、分类与图标缓存（保留配置与密码）。
func (s *Server) handlePurgeAll(c *gin.Context) {
	if err := s.store.PurgeAll(); err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "清空数据失败")
		return
	}
	ok(c, gin.H{"purged": true})
}

// handleHDIconsSearch 后台图标选择器用：在本地索引缓存里搜条目名，不发网络请求。
// 先搜 HD-Icons（中文查询会先经关键词/别名表翻成英文线索），再把 nasicon 的结果以
// 「nasicon:<文件名>」前缀追加在后面，所以一个输入框能搜到两个来源。
func (s *Server) handleHDIconsSearch(c *gin.Context) {
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value <= 100 {
			limit = value
		}
	}
	query := c.Query("q")
	items := make([]string, 0, limit*2)
	cached := false
	if index := s.hdIndexCached(); index != nil {
		cached = true
		items = append(items, hdicons.Suggest(index.Items, query, limit)...)
	}
	if index := s.nasIndexCached(); index != nil {
		cached = true
		for _, name := range nasicon.Suggest(index.Items, query, limit) {
			items = append(items, nasiconPrefix+name)
		}
	}
	ok(c, gin.H{"items": items, "count": len(items), "cached": cached})
}

// handleHDIconsIcon 输出索引里某个条目的图标（后台预览用），复用与抓取完全相同的下载链路。
// 名称带「nasicon:」前缀时走 nasicon.top，否则按 HD-Icons 解析。
func (s *Server) handleHDIconsIcon(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("name"))
	if filename, ok := strings.CutPrefix(raw, nasiconPrefix); ok {
		index := s.nasIndexCached()
		if index == nil {
			fail(c, http.StatusServiceUnavailable, "index_missing", "本地还没有 nasicon 索引缓存，先抓取一次图标即可自动建立")
			return
		}
		item := nasicon.Resolve(index.Items, filename)
		if item.Filename == "" {
			fail(c, http.StatusNotFound, "unknown_icon", "nasicon 索引里没有这个图标")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
		defer cancel()
		data, contentType, err := s.nas.FetchIcon(ctx, item.Filename)
		if err != nil {
			fail(c, http.StatusBadGateway, "icon_unreachable", "图标下载失败："+err.Error())
			return
		}
		servePreviewIcon(c, data, contentType)
		return
	}
	index := s.hdIndexCached()
	if index == nil {
		fail(c, http.StatusServiceUnavailable, "index_missing", "本地还没有图标索引缓存，请先在设置里测试一次连通性")
		return
	}
	name := hdicons.Resolve(index.Items, raw)
	if name == "" {
		fail(c, http.StatusNotFound, "unknown_icon", "索引里没有这个图标")
		return
	}
	assetPath := hdicons.AssetPath(index.Items, name)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	for _, mirror := range hdicons.MergeMirrors(s.hd.Mirrors(), index.Mirror) {
		data, contentType, err := s.hd.FetchIcon(ctx, mirror, assetPath)
		if err != nil {
			continue
		}
		servePreviewIcon(c, data, contentType)
		return
	}
	fail(c, http.StatusBadGateway, "icon_unreachable", "图标下载失败，请检查镜像前缀")
}

// servePreviewIcon 与落盘一致地处理（去最外圈细边框、缩小大图），保证「预览所见 = 保存所得」。
func servePreviewIcon(c *gin.Context, data []byte, contentType string) {
	if trimmed := favicon.TrimFrame(data); trimmed != nil {
		data = trimmed
	}
	if small, ct, ok := favicon.Shrink(data, favicon.MaxIconSize); ok {
		data, contentType = small, ct
	}
	c.Header("Content-Security-Policy", iconSandbox)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, contentType, data)
}
