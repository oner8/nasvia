// Package server 提供 NASVIA 的 HTTP 接口与内嵌前端服务。
package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/auth"
	"github.com/oner8/nasvia/internal/config"
	"github.com/oner8/nasvia/internal/favicon"
	"github.com/oner8/nasvia/internal/hdicons"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/nasicon"
	"github.com/oner8/nasvia/internal/netdetect"
	"github.com/oner8/nasvia/internal/store"
	"github.com/oner8/nasvia/internal/suggest"
	"github.com/oner8/nasvia/internal/visibility"
	webui "github.com/oner8/nasvia/web"
)

// Server 持有依赖并暴露 http.Handler。
type Server struct {
	cfg      *config.Config
	store    *store.Store
	sessions *auth.Manager
	limiter  *auth.Limiter
	fetcher  *favicon.Fetcher
	// hd 负责 HD-Icons 的索引缓存与图标下载（按站点名匹配圆角图标）。
	hd *hdicons.Client
	// hdIndexMu 串行化「读缓存 → 拉取索引 → 写缓存」，避免多站点同时抓取时重复下载。
	hdIndexMu sync.Mutex
	// nas 负责 nasicon.top 的索引缓存与图标下载（HD-Icons 之后的中文名/域名兜底来源）。
	nas *nasicon.Client
	// nasIndexMu 串行化 nasicon 的「读缓存 → 拉取索引 → 写缓存」。
	nasIndexMu sync.Mutex
	// hdCache / nasCache 解析好的索引缓存（按文件修改时间失效），避免每次读盘 + 反序列化。
	hdCache  fileCache[hdicons.Index]
	nasCache fileCache[nasicon.Index]
	// hdFailedAt / nasFailedAt 最近一次拉取索引失败的时间（分别受 hdIndexMu / nasIndexMu 保护）。
	// 失败后 indexRetryAfter 内不再重试：国内直连 GitHub 常常不通，否则每个图标任务都要在锁里白等一次超时。
	hdFailedAt  time.Time
	nasFailedAt time.Time
	// egress 家庭公网出口（DDNS 域名 / auto 由后台定期解析），按访客 IP 判定内外网用。
	egress *netdetect.Egress
	// suggest 联网搜索框的联想词代理。
	suggest *suggest.Client
	engine  *gin.Engine

	assets fs.FS
	// iconSem 限制并发图标抓取数量，避免首次加载打爆来源站点。
	iconSem chan struct{}
	// inFlight 记录正在抓取的站点，避免重复任务。
	inFlight sync.Map
}

// New 构造服务实例。
func New(cfg *config.Config, st *store.Store) (*Server, error) {
	assets, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded assets: %w", err)
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		sessions: auth.NewManager(auth.DefaultTTL),
		limiter:  auth.NewLimiter(5, 60*time.Second),
		fetcher:  favicon.NewFetcher(favicon.ParseSources(st.Setting(model.SettingFaviconSources, cfg.FaviconSources))),
		hd:       hdicons.NewClient(hdicons.ParseMirrors(st.Setting(model.SettingHDIconsMirrors, cfg.HDIconsMirrors))),
		nas:      nasicon.NewClient(st.Setting(model.SettingNASIconBase, cfg.NASIconBase)),
		egress:   netdetect.NewEgress(),
		suggest:  suggest.NewClient(),
		assets:   assets,
		iconSem:  make(chan struct{}, 4),
	}
	engine, err := s.buildRouter()
	if err != nil {
		return nil, err
	}
	s.engine = engine
	s.egress.SetEntries(s.homeEgressRaw())
	go s.egress.Run(5 * time.Minute)
	go s.sessionGC()
	return s, nil
}

// Handler 返回 HTTP 处理器。
func (s *Server) Handler() http.Handler { return s.engine }

// Sessions 暴露会话管理器（测试使用）。
func (s *Server) Sessions() *auth.Manager { return s.sessions }

func (s *Server) sessionGC() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.sessions.GC()
	}
}

func (s *Server) buildRouter() (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), securityHeaders(), sameOriginWrites())
	r.RedirectTrailingSlash = false
	// 只信任来自这些地址的 X-Forwarded-For / X-Real-IP（gin 默认信任所有来源，客户端可随意伪造 IP）。
	// 反代（如 NPM 经 WireGuard 转发）在其中时，c.ClientIP() 才是访客真实 IP，内外网判定与登录限速都靠它。
	if err := r.SetTrustedProxies(netdetect.SplitList(s.cfg.TrustedProxies)); err != nil {
		return nil, fmt.Errorf("NASVIA_TRUSTED_PROXIES 格式不正确：%w", err)
	}

	api := r.Group("/api")
	{
		api.GET("/health", s.handleHealth)
		api.GET("/config", s.handleConfig)
		api.POST("/auth/login", s.handleLogin)
		api.POST("/auth/logout", s.handleLogout)
		api.GET("/auth/session", s.handleSession)

		api.GET("/sites", s.handleListSites)
		api.GET("/sites/icon-status", s.handleListIconStatuses)
		api.GET("/categories", s.handleListCategories)
		api.GET("/sites/:id/icon", s.handleSiteIcon)
		api.GET("/suggest", s.handleSuggest)

		admin := api.Group("/admin", s.requireAdmin())
		{
			admin.GET("/sites", s.handleAdminListSites)
			admin.GET("/sites/icon-status", s.handleAdminListIconStatuses)
			admin.POST("/sites", s.handleCreateSite)
			admin.POST("/sites/batch", s.handleBatchSites)
			admin.POST("/sites/purge", s.handlePurgeSites)
			admin.POST("/sites/reorder", s.handleReorderSites)
			admin.PUT("/sites/:id", s.handleUpdateSite)
			admin.DELETE("/sites/:id", s.handleDeleteSite)
			admin.POST("/sites/:id/move", s.handleMoveSite)
			admin.POST("/sites/:id/refetch-icon", s.handleRefetchIcon)

			admin.GET("/categories", s.handleAdminListCategories)
			admin.POST("/categories", s.handleCreateCategory)
			admin.POST("/categories/purge", s.handlePurgeCategories)
			admin.PUT("/categories/:id", s.handleUpdateCategory)
			admin.DELETE("/categories/:id", s.handleDeleteCategory)
			admin.POST("/categories/:id/move", s.handleMoveCategory)

			admin.GET("/settings", s.handleGetSettings)
			admin.POST("/hdicons/test", s.handleTestHDIcons)
			admin.GET("/hdicons/search", s.handleHDIconsSearch)
			admin.GET("/hdicons/icon", s.handleHDIconsIcon)
			admin.PUT("/settings", s.handleUpdateSettings)
			admin.PUT("/config", s.handleUpdateSettings)
			admin.POST("/password", s.handleChangePassword)
			admin.POST("/purge", s.handlePurgeAll)
		}
	}

	r.NoRoute(s.serveStatic)
	return r, nil
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	}
}

// sameOriginWrites 拒绝跨源的写请求（CSRF）。同根域名下的其它子站与本站同属一个 site，
// SameSite=Lax 的会话 Cookie 照样会被带上，所以按 Sec-Fetch-Site / Origin 只放行同源页面发起的请求。
// GET/HEAD/OPTIONS，以及不带这两个头的非浏览器请求（curl、脚本）不受影响。
func sameOriginWrites() gin.HandlerFunc {
	guard := http.NewCrossOriginProtection()
	return func(c *gin.Context) {
		if err := guard.Check(c.Request); err != nil {
			fail(c, http.StatusForbidden, "cross_origin", "拒绝跨站请求，请在本站页面内操作")
			return
		}
		c.Next()
	}
}

// serveStatic 提供内嵌前端资源，并对未知路径回落 index.html（SPA）。
func (s *Server) serveStatic(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	reqPath := c.Request.URL.Path
	if strings.HasPrefix(reqPath, "/api/") {
		fail(c, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	name := strings.TrimPrefix(reqPath, "/")
	if name != "" {
		if f, err := s.assets.Open(name); err == nil {
			_ = f.Close()
			if strings.HasPrefix(reqPath, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
			http.FileServer(http.FS(s.assets)).ServeHTTP(c.Writer, c.Request)
			return
		}
		if strings.HasPrefix(reqPath, "/assets/") {
			fail(c, http.StatusNotFound, "not_found", "静态资源不存在")
			return
		}
	}
	data, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		c.String(http.StatusServiceUnavailable, "前端资源未内嵌，请先构建 web/dist")
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// ---------------------------------------------------------------- 通用工具

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, apiError{Code: code, Message: message})
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

func parseID(c *gin.Context) (uint, bool) {
	raw := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil || id == 0 {
		fail(c, http.StatusBadRequest, "invalid_id", "无效的 ID")
		return 0, false
	}
	return id, true
}

// ---------------------------------------------------------------- 鉴权辅助

func (s *Server) authenticated(c *gin.Context) bool {
	token, err := c.Cookie(auth.CookieName)
	if err != nil {
		return false
	}
	return s.sessions.Valid(token)
}

// passwordConfigured 数据库散列优先，其次环境变量。
func (s *Server) passwordConfigured() bool {
	hash, err := s.store.PasswordHash()
	if err == nil && hash != "" {
		return true
	}
	return strings.TrimSpace(s.cfg.Password) != ""
}

func (s *Server) authMode() string {
	return model.NormalizeAuthMode(s.store.Setting(model.SettingAuthMode, s.cfg.AuthMode))
}

func (s *Server) lanCIDRs() []string {
	return netdetect.SplitList(s.store.Setting(model.SettingLANCIDRs, s.cfg.LANCIDRs))
}

func (s *Server) homeEgressRaw() string {
	return s.store.Setting(model.SettingHomeEgress, s.cfg.HomeEgress)
}

// network 按访客真实 IP 判定内外网（结果只决定站点链接用哪个地址，不参与鉴权）。
func (s *Server) network(c *gin.Context) netdetect.Result {
	return netdetect.Classify(c.ClientIP(), s.lanCIDRs(), s.egress)
}

func (s *Server) siteTitle() string {
	return s.store.Setting(model.SettingSiteTitle, s.cfg.SiteTitle)
}

// requireAdmin 后台接口守卫：未设置密码时明确拒绝，未登录时要求登录。
func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.passwordConfigured() {
			fail(c, http.StatusForbidden, "password_not_set",
				"未设置密码，仅限内网访问：请设置 NASVIA_PASSWORD 环境变量，或在本机先设置密码后再使用后台")
			return
		}
		if !s.authenticated(c) {
			fail(c, http.StatusUnauthorized, "admin_unauthorized", "请先登录后再访问管理接口")
			return
		}
		c.Next()
	}
}

// readGate 前台读取守卫：private 模式下必须登录；public 模式下允许匿名（仅能看到公开内容）。
func (s *Server) readGate(c *gin.Context) bool {
	if s.authMode() == model.AuthModePrivate && !s.authenticated(c) {
		fail(c, http.StatusUnauthorized, "login_required", "本站已设为「私密」模式，请先登录")
		return false
	}
	return true
}

// ---------------------------------------------------------------- 图标抓取

type iconQueueResult uint8

const (
	iconQueued iconQueueResult = iota
	iconAlreadyQueued
	iconUnavailable
)

// queueIconFetch 校验、占位并启动单个抓取任务；reset 仅供手动重抓，在任务启动前清空旧图标。
func (s *Server) queueIconFetch(site model.Site, reset bool) (iconQueueResult, error) {
	target := iconMatchTarget(site)
	if target == "" || !s.fetcher.Sources().Any() {
		return iconUnavailable, nil
	}
	key := site.ID
	if _, loaded := s.inFlight.LoadOrStore(key, struct{}{}); loaded {
		return iconAlreadyQueued, nil
	}
	if reset {
		if err := s.store.MarkIconPending(site.ID); err != nil {
			s.inFlight.Delete(key)
			return iconUnavailable, err
		}
	}
	go func() {
		defer s.inFlight.Delete(key)
		s.iconSem <- struct{}{}
		defer func() { <-s.iconSem }()
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		// HD-Icons 优先，其次 nasicon（中文名/域名兜底）；都匹配不到再回落 favicon 链路。
		sources := s.fetcher.Sources()
		if sources.HDIcons {
			if res := s.fetchHDIcons(ctx, site); res != nil {
				s.applyIconResult(site, res)
				return
			}
		}
		if sources.NASIcon {
			if res := s.fetchNASIcon(ctx, site); res != nil {
				s.applyIconResult(site, res)
				return
			}
		}
		res, err := s.fetcher.Fetch(ctx, target)
		if err != nil {
			s.storePlaceholderIcon(site)
			return
		}
		s.applyIconResult(site, res)
	}()
	return iconQueued, nil
}

// triggerIconFetch 用于创建、编辑和图标缺失时的自动抓取；无法抓取时立即落回占位图，避免永久 pending。
func (s *Server) triggerIconFetch(site model.Site) iconQueueResult {
	result, _ := s.queueIconFetch(site, false)
	if result == iconUnavailable {
		s.storePlaceholderIcon(site)
	}
	return result
}

// applyIconResult 落盘抓到的图标：内容哈希命名 + 记录到站点 + 按域名缓存。
func (s *Server) applyIconResult(site model.Site, res *favicon.Result) {
	if res == nil || len(res.Data) == 0 {
		s.storePlaceholderIcon(site)
		return
	}
	// 有些来源的图标其实是网页截图，最外圈带 1px 灰线（深色主题下像黑边）→ 裁掉（见 favicon.TrimFrame）
	data, contentType := res.Data, res.ContentType
	if trimmed := favicon.TrimFrame(data); trimmed != nil {
		data = trimmed
	}
	// 大图（HD-Icons 原图 1024px）缩到首页够用的尺寸再落盘，省流量、加快加载
	if small, ct, ok := favicon.Shrink(data, favicon.MaxIconSize); ok {
		data, contentType = small, ct
	}
	hash := favicon.Hash(data)
	ext := favicon.Ext(contentType)
	if _, err := s.store.WriteIconFile(hash, ext, data); err != nil {
		s.storePlaceholderIcon(site)
		return
	}
	_ = s.store.SetSiteIcon(site.ID, hash, contentType, res.Source)
	if res.Domain != "" {
		_ = s.store.SaveIconCache(&model.IconCache{
			Domain:      res.Domain,
			Hash:        hash,
			Ext:         ext,
			ContentType: contentType,
			Source:      res.Source,
		})
	}
}

// fetchHDIcons 尝试用 HD-Icons 匹配该站点；匹配不到或取不到返回 nil（交给 favicon 兜底）。
func (s *Server) fetchHDIcons(ctx context.Context, site model.Site) *favicon.Result {
	if s.hd == nil {
		return nil
	}
	index := s.hdIndex(ctx)
	if index == nil {
		return nil
	}
	// 手动指定的条目名优先；解析不到（索引里没有/写错了）再退回自动匹配。
	name := ""
	switch manual := strings.TrimSpace(site.IconName); {
	case strings.HasPrefix(manual, nasiconPrefix):
		return nil // 手动指定的是 nasicon 条目，交给 fetchNASIcon
	case manual != "":
		name = hdicons.Resolve(index.Items, manual)
	}
	if name == "" {
		name = hdicons.Match(site.Name, iconMatchTarget(site), index.Items)
	}
	if name == "" {
		return nil
	}
	assetPath := hdicons.AssetPath(index.Items, name)
	if assetPath == "" {
		return nil
	}
	// 依次尝试：配置的镜像优先，索引里记录的那个镜像兜底；直到有一个下载成功。
	for _, mirror := range hdicons.MergeMirrors(s.hd.Mirrors(), index.Mirror) {
		data, contentType, err := s.hd.FetchIcon(ctx, mirror, assetPath)
		if err != nil {
			continue
		}
		return &favicon.Result{Data: data, ContentType: contentType, Source: "hdicons:" + name}
	}
	return nil
}

// indexRetryAfter 图标索引拉取失败后的退避时间。
const indexRetryAfter = 10 * time.Minute

// resetIndexBackoff 换了镜像 / 地址后立即允许重试拉取索引。
func (s *Server) resetIndexBackoff() {
	s.hdIndexMu.Lock()
	s.hdFailedAt = time.Time{}
	s.hdIndexMu.Unlock()
	s.nasIndexMu.Lock()
	s.nasFailedAt = time.Time{}
	s.nasIndexMu.Unlock()
}

// hdIndex 取索引：新鲜缓存直接用；过期则拉取并写缓存；拉取失败退回过期缓存；都没有则 nil。
func (s *Server) hdIndex(ctx context.Context) *hdicons.Index {
	path := s.hdIndexPath()
	s.hdIndexMu.Lock()
	defer s.hdIndexMu.Unlock()

	cached := s.hdCache.load(path, hdicons.ReadCache)
	if cached != nil && cached.Fresh(time.Now()) {
		return cached
	}
	if time.Since(s.hdFailedAt) < indexRetryAfter {
		return cached
	}
	index, err := s.hd.FetchIndex(ctx)
	if err != nil {
		s.hdFailedAt = time.Now()
		return cached // 镜像不可达时用过期缓存兜底，保证离线也能匹配
	}
	s.hdFailedAt = time.Time{}
	_ = hdicons.WriteCache(path, index)
	return index
}

// hdIndexPath 索引缓存文件位置（放 data 目录里，备份/迁移时随行）。
func (s *Server) hdIndexPath() string {
	return filepath.Join(s.cfg.DataDir, "hdicons.json")
}

// hdIndexCached 只读本地索引缓存（不发网络请求），供后台展示「自动匹配到哪个图标」。
func (s *Server) hdIndexCached() *hdicons.Index {
	return s.hdCache.load(s.hdIndexPath(), hdicons.ReadCache)
}

// iconMatchTarget 图标匹配用的地址：优先外网地址，为空时用内网地址。
func iconMatchTarget(site model.Site) string {
	if target := strings.TrimSpace(site.URL); target != "" {
		return target
	}
	return strings.TrimSpace(site.LanURL)
}

// ---------------------------------------------------------------- nasicon（HD-Icons 之后的兜底来源）

// nasiconPrefix 手动指定 nasicon 条目时的前缀（裸名仍按 HD-Icons 解析，既有数据不受影响）。
const nasiconPrefix = "nasicon:"

// nasIndex 取 nasicon 索引：新鲜缓存直接用；过期则拉取并写缓存；拉取失败退回过期缓存；都没有则 nil。
func (s *Server) nasIndex(ctx context.Context) *nasicon.Index {
	path := s.nasIndexPath()
	s.nasIndexMu.Lock()
	defer s.nasIndexMu.Unlock()

	cached := s.nasCache.load(path, nasicon.ReadCache)
	if cached != nil && cached.Fresh(time.Now()) {
		return cached
	}
	if time.Since(s.nasFailedAt) < indexRetryAfter {
		return cached
	}
	index, err := s.nas.FetchIndex(ctx)
	if err != nil {
		s.nasFailedAt = time.Now()
		return cached // 站点不可达时用过期缓存兜底，保证离线也能按已缓存的名字匹配
	}
	s.nasFailedAt = time.Time{}
	_ = nasicon.WriteCache(path, index)
	return index
}

// nasIndexPath nasicon 索引缓存文件位置（放 data 目录里，备份/迁移时随行）。
func (s *Server) nasIndexPath() string {
	return filepath.Join(s.cfg.DataDir, "nasicon.json")
}

// nasIndexCached 只读本地索引缓存（不发网络请求）。
func (s *Server) nasIndexCached() *nasicon.Index {
	return s.nasCache.load(s.nasIndexPath(), nasicon.ReadCache)
}

// fetchNASIcon 用 nasicon.top 匹配该站点（中文名 / 英文名 / 域名）；匹配不到或取不到返回 nil。
// 手动指定「nasicon:<文件名>」时只认那一条，不会再跑回自动匹配。
func (s *Server) fetchNASIcon(ctx context.Context, site model.Site) *favicon.Result {
	if s.nas == nil {
		return nil
	}
	index := s.nasIndex(ctx)
	if index == nil {
		return nil
	}
	manual := strings.TrimSpace(site.IconName)
	item := nasicon.Item{}
	switch {
	case strings.HasPrefix(manual, nasiconPrefix):
		item = nasicon.Resolve(index.Items, strings.TrimPrefix(manual, nasiconPrefix))
	case manual == "":
		item = nasicon.Match(site.Name, iconMatchTarget(site), index.Items)
	}
	if item.Filename == "" {
		return nil
	}
	data, contentType, err := s.nas.FetchIcon(ctx, item.Filename)
	if err != nil {
		return nil
	}
	return &favicon.Result{Data: data, ContentType: contentType, Source: nasiconPrefix + item.Filename}
}

// autoIconName 只读本地索引缓存，算出「自动匹配会选中哪个图标」：HD-Icons 优先，其次 nasicon。
// 后台列表用它提示「留空会匹配到哪个」，全程不发网络请求。
func (s *Server) autoIconName(site model.Site) string {
	target := iconMatchTarget(site)
	if index := s.hdIndexCached(); index != nil {
		if name := hdicons.Match(site.Name, target, index.Items); name != "" {
			return name
		}
	}
	if index := s.nasIndexCached(); index != nil {
		if item := nasicon.Match(site.Name, target, index.Items); item.Filename != "" {
			return nasiconPrefix + item.Filename
		}
	}
	return ""
}

// storePlaceholderIcon 在抓取失败时也把内置占位图落到本地缓存，
// 这样图标接口始终能命中本地文件（避免每次请求都重新抓取），同时保留 failed 状态供后台重试。
func (s *Server) storePlaceholderIcon(site model.Site) {
	placeholder := favicon.Placeholder(site.Name, 96)
	if len(placeholder) == 0 {
		_ = s.store.MarkIconFailed(site.ID)
		return
	}
	hash := favicon.Hash(placeholder)
	if _, err := s.store.WriteIconFile(hash, ".svg", placeholder); err != nil {
		_ = s.store.MarkIconFailed(site.ID)
		return
	}
	_ = s.store.SetSiteIconFailed(site.ID, hash, "image/svg+xml")
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// iconSandbox 图标响应的 CSP：图标里的 SVG 即使被当作页面直接打开，也不能执行脚本、加载外部资源
// （远程抓来的 SVG 可能夹带脚本，而它是以本站同源身份输出的）。
const iconSandbox = "default-src 'none'; style-src 'unsafe-inline'; sandbox"

// serveCachedIcon 输出磁盘上的图标文件；返回是否命中。
// immutable=true 表示请求地址带的就是当前内容版本（?v=哈希前缀），可以让浏览器长期缓存：
// 图标一变，地址里的版本跟着变，不会用到旧图。否则每次回源校验（ServeFile 会给 Last-Modified）。
func (s *Server) serveCachedIcon(c *gin.Context, hash string, immutable bool) bool {
	path, found := s.store.FindIconFile(hash)
	if !found {
		return false
	}
	c.Header("Content-Type", mimeForExt(strings.ToLower(path[strings.LastIndex(path, "."):])))
	c.Header("Content-Security-Policy", iconSandbox)
	if immutable {
		c.Header("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "private, no-cache")
	}
	c.File(path)
	return true
}

// ShrinkStoredIcons 把旧版本落盘的大图标（例如 1024px 的 HD-Icons 原图）就地缩小，文件名不变。
// 启动时在后台跑一次；已经够小的图标会被跳过，所以重复执行几乎没有开销。
func (s *Server) ShrinkStoredIcons() (int, error) {
	dir := s.store.IconsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	shrunk := 0
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		default:
			continue
		}
		oldPath := filepath.Join(dir, name)
		data, err := os.ReadFile(oldPath)
		if err != nil {
			continue
		}
		small, _, ok := favicon.Shrink(data, favicon.MaxIconSize)
		if !ok {
			continue
		}
		hash := strings.TrimSuffix(name, filepath.Ext(name))
		newPath := filepath.Join(dir, hash+".png")
		tmp, err := os.CreateTemp(dir, ".shrink-*.tmp")
		if err != nil {
			continue
		}
		_, werr := tmp.Write(small)
		cerr := tmp.Close()
		if werr != nil || cerr != nil || os.Rename(tmp.Name(), newPath) != nil {
			_ = os.Remove(tmp.Name())
			continue
		}
		if newPath != oldPath {
			_ = os.Remove(oldPath)
		}
		shrunk++
	}
	return shrunk, nil
}

func (s *Server) servePlaceholder(c *gin.Context, name string) {
	c.Header("Content-Type", "image/svg+xml")
	c.Header("Content-Security-Policy", iconSandbox)
	// 占位图不缓存：图标抓好后是同一个 URL，缓存住占位图会让新站点一直显示字母方块。
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/svg+xml", favicon.Placeholder(name, 96))
}

// categoryIndex 建立 id -> 分类 的索引。
func categoryIndex(cats []model.Category) map[uint]*model.Category {
	out := make(map[uint]*model.Category, len(cats))
	for i := range cats {
		out[cats[i].ID] = &cats[i]
	}
	return out
}

func categoryOf(site model.Site, index map[uint]*model.Category) *model.Category {
	if site.CategoryID == nil {
		return nil
	}
	return index[*site.CategoryID]
}

var _ = visibility.SiteVisible // 保留可见性包的显式引用，便于阅读路由语义
