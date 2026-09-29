package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oner8/nasvia/internal/auth"
	"github.com/oner8/nasvia/internal/config"
	"github.com/oner8/nasvia/internal/hdicons"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/server"
	"github.com/oner8/nasvia/internal/store"
)

type response struct {
	status int
	body   []byte
	header http.Header
}

type env struct {
	ts     *httptest.Server
	st     *store.Store
	cfg    *config.Config
	dir    string
	client *http.Client
}

func newEnv(t *testing.T, password, authMode string) *env {
	return newEnvWith(t, password, authMode, nil)
}

// newEnvWith 与 newEnv 相同，但可以在构造服务前改配置（图标来源、镜像前缀、nasicon 地址等）。
func newEnvWith(t *testing.T, password, authMode string, mutate func(*config.Config)) *env {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nasvia.db")
	st, err := store.Open(dbPath, filepath.Join(dir, "icons"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		Bind:           "127.0.0.1",
		Port:           3720,
		DataDir:        dir,
		DBPath:         dbPath,
		AuthMode:       authMode,
		SiteTitle:      "测试导航",
		Password:       password,
		FaviconSources: "site",
	}
	if mutate != nil {
		mutate(cfg)
	}
	if err := st.EnsureDefaults(map[string]string{
		model.SettingAuthMode:       cfg.AuthMode,
		model.SettingSiteTitle:      cfg.SiteTitle,
		model.SettingFaviconSources: cfg.FaviconSources,
		model.SettingHDIconsMirrors: cfg.HDIconsMirrors,
		model.SettingNASIconBase:    cfg.NASIconBase,
	}); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if _, err := st.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv, err := server.New(cfg, st)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &env{ts: ts, st: st, cfg: cfg, dir: dir, client: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func (e *env) do(t *testing.T, method, path string, payload any) *response {
	t.Helper()
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return &response{status: resp.StatusCode, body: body, header: resp.Header}
}

func (e *env) json(t *testing.T, method, path string, payload any, out any) *response {
	t.Helper()
	resp := e.do(t, method, path, payload)
	if out != nil && len(resp.body) > 0 {
		if err := json.Unmarshal(resp.body, out); err != nil {
			t.Fatalf("解析 %s %s 响应失败：%v（%s）", method, path, err, string(resp.body))
		}
	}
	return resp
}

type siteListResponse struct {
	Items []struct {
		ID                  uint   `json:"id"`
		Name                string `json:"name"`
		Visibility          string `json:"visibility"`
		EffectiveVisibility string `json:"effective_visibility"`
		Icon                string `json:"icon"`
		IconState           string `json:"icon_state"`
		IconSource          string `json:"icon_source"`
		IconAuto            string `json:"icon_auto"`
		LanURL              string `json:"lan_url"`
		Description         string `json:"description"`
	} `json:"items"`
	Total         int  `json:"total"`
	Authenticated bool `json:"authenticated"`
}

type categoryListResponse struct {
	Items []struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
		SiteCount  int    `json:"site_count"`
	} `json:"items"`
	Total int `json:"total"`
}

func (e *env) login(t *testing.T, password string) *response {
	return e.json(t, http.MethodPost, "/api/auth/login", map[string]string{"password": password}, nil)
}

func TestSeedDataIsServedInPrivateMode(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)

	var list siteListResponse
	resp := e.json(t, http.MethodGet, "/api/sites", nil, &list)
	if resp.status != http.StatusUnauthorized {
		t.Fatalf("private 模式未登录应返回 401，得到 %d", resp.status)
	}

	index := e.do(t, http.MethodGet, "/", nil)
	if index.status != http.StatusOK || !strings.Contains(string(index.body), `<div id="root">`) {
		t.Fatalf("首页应仍返回页面以便渲染登录视图，状态 %d", index.status)
	}

	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("登录应成功，得到 %d（%s）", resp.status, string(resp.body))
	}

	e.json(t, http.MethodGet, "/api/sites", nil, &list)
	if list.Total < 10 {
		t.Fatalf("登录后应看到 ≥10 条演示站点，得到 %d", list.Total)
	}

	var dbCount int64
	if err := e.st.DB().Model(&model.Site{}).Count(&dbCount).Error; err != nil {
		t.Fatal(err)
	}
	if int64(list.Total) != dbCount {
		t.Fatalf("登录后应看到全部站点：接口 %d，数据库 %d", list.Total, dbCount)
	}

	var cats categoryListResponse
	e.json(t, http.MethodGet, "/api/categories", nil, &cats)
	if cats.Total < 3 {
		t.Fatalf("演示分类应 ≥3，得到 %d", cats.Total)
	}
}

func TestPublicModeHidesPrivateContent(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)

	resp := e.json(t, http.MethodGet, "/api/sites", nil, nil)
	if resp.status != http.StatusOK {
		t.Fatalf("public 模式匿名应返回 200，得到 %d", resp.status)
	}
	raw := string(resp.body)

	var anon siteListResponse
	if err := json.Unmarshal(resp.body, &anon); err != nil {
		t.Fatal(err)
	}

	var total int64
	if err := e.st.DB().Model(&model.Site{}).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if int64(anon.Total) >= total {
		t.Fatalf("匿名可见站点应少于总数：%d / %d", anon.Total, total)
	}

	// 私密站点的名称、描述、地址都不得出现在响应里
	rows, err := e.st.Sites()
	if err != nil {
		t.Fatal(err)
	}
	privateCount := 0
	for _, row := range rows {
		if row.Visibility != model.VisibilityPrivate {
			continue
		}
		privateCount++
		for _, needle := range []string{row.Name, row.Description, row.LanURL, row.URL, row.Tags} {
			if needle == "" {
				continue
			}
			// 带上 JSON 引号，避免与公开站点的前缀（如 192.168.1.1 与 192.168.1.10）误判
			quoted := strconv.Quote(needle)
			if strings.Contains(raw, quoted) {
				t.Fatalf("私密内容泄露：响应包含 %q", needle)
			}
		}
	}
	if privateCount == 0 {
		t.Fatal("演示数据应包含私密站点")
	}

	// 搜索接口同样不能泄露
	privateName := ""
	for _, row := range rows {
		if row.Visibility == model.VisibilityPrivate {
			privateName = row.Name
			break
		}
	}
	q := e.json(t, http.MethodGet, "/api/sites?q="+privateName, nil, nil)
	var searched siteListResponse
	if err := json.Unmarshal(q.body, &searched); err != nil {
		t.Fatal(err)
	}
	if searched.Total != 0 {
		t.Fatalf("匿名搜索私密站点应为空，得到 %d 条", searched.Total)
	}
	if strings.Contains(string(q.body), privateName) {
		t.Fatal("匿名搜索结果泄露了私密站点")
	}

	// 分类列表也不应包含私密分类
	var cats categoryListResponse
	e.json(t, http.MethodGet, "/api/categories", nil, &cats)
	for _, c := range cats.Items {
		if c.Visibility == model.VisibilityPrivate {
			t.Fatalf("私密分类不应出现在匿名分类列表中：%+v", c)
		}
	}

	// 登录后过滤解除，说明过滤由身份决定而非模式写死
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("登录失败：%d", resp.status)
	}
	var authed siteListResponse
	e.json(t, http.MethodGet, "/api/sites", nil, &authed)
	if int64(authed.Total) != total {
		t.Fatalf("登录后应看到全部 %d 条，得到 %d", total, authed.Total)
	}
}

func TestAdminWithoutPasswordIsRejected(t *testing.T) {
	e := newEnv(t, "", model.AuthModePublic)

	resp := e.json(t, http.MethodGet, "/api/admin/sites", nil, nil)
	if resp.status != http.StatusForbidden {
		t.Fatalf("未设置密码时后台应返回 403，得到 %d", resp.status)
	}
	if !strings.Contains(string(resp.body), "未设置密码，仅限内网访问") {
		t.Fatalf("响应应给出明确说明，得到 %s", string(resp.body))
	}
	if !strings.Contains(string(resp.body), `"code":"password_not_set"`) {
		t.Fatalf("应返回 code=password_not_set，得到 %s", string(resp.body))
	}
	if resp := e.do(t, http.MethodGet, "/", nil); resp.status != http.StatusOK {
		t.Fatalf("首页仍应可访问，得到 %d", resp.status)
	}
	if resp := e.do(t, http.MethodGet, "/api/sites", nil); resp.status != http.StatusOK {
		t.Fatalf("公开模式的站点列表仍应可访问，得到 %d", resp.status)
	}
	if resp := e.json(t, http.MethodPost, "/api/auth/login", map[string]string{"password": "x"}, nil); resp.status != http.StatusForbidden {
		t.Fatalf("未设置密码时登录也应被拒绝，得到 %d", resp.status)
	}
}

func TestIconNotEnumerableForPrivateSite(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)

	category := &model.Category{Name: "私密", Visibility: model.VisibilityPrivate}
	if err := e.st.CreateCategory(category); err != nil {
		t.Fatal(err)
	}
	site := &model.Site{Name: "Vault", URL: "https://vault.example.com", CategoryID: &category.ID, Visibility: model.VisibilityPrivate}
	if err := e.st.CreateSite(site); err != nil {
		t.Fatal(err)
	}

	resp := e.do(t, http.MethodGet, "/api/sites/"+itoa(site.ID)+"/icon", nil)
	if resp.status != http.StatusNotFound && resp.status != http.StatusUnauthorized {
		t.Fatalf("未登录不应拿到私密站点图标，得到 %d", resp.status)
	}
	if strings.Contains(resp.header.Get("Content-Type"), "image") {
		t.Fatalf("未登录不应返回图片内容：%s", resp.header.Get("Content-Type"))
	}

	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}
	authed := e.do(t, http.MethodGet, "/api/sites/"+itoa(site.ID)+"/icon", nil)
	if authed.status != http.StatusOK {
		t.Fatalf("登录后应能取到图标（占位图），得到 %d", authed.status)
	}
	if !strings.Contains(authed.header.Get("Content-Type"), "svg") {
		t.Fatalf("无缓存时应返回 SVG 占位图，得到 %s", authed.header.Get("Content-Type"))
	}
}

func TestPrivateCategoryCapsSiteVisibility(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)

	privateCat := &model.Category{Name: "内部", Visibility: model.VisibilityPrivate}
	if err := e.st.CreateCategory(privateCat); err != nil {
		t.Fatal(err)
	}
	inheritSite := &model.Site{Name: "继承站点", URL: "https://a.example.com", CategoryID: &privateCat.ID, Visibility: model.VisibilityInherit}
	// 历史数据：私密分类里存在显式公开的站点（新规则下 API 已拒绝创建，这里直接落库模拟）。
	legacyPublicSite := &model.Site{Name: "历史公开站点", URL: "https://b.example.com", CategoryID: &privateCat.ID, Visibility: model.VisibilityPublic}
	noCatSite := &model.Site{Name: "无分类站点", URL: "https://c.example.com", Visibility: model.VisibilityInherit}
	for _, site := range []*model.Site{inheritSite, legacyPublicSite, noCatSite} {
		if err := e.st.CreateSite(site); err != nil {
			t.Fatal(err)
		}
	}

	var list siteListResponse
	e.json(t, http.MethodGet, "/api/sites", nil, &list)
	names := map[string]bool{}
	for _, item := range list.Items {
		names[item.Name] = true
	}
	if names["继承站点"] {
		t.Fatal("私密分类下继承站点不应对匿名可见")
	}
	if names["历史公开站点"] {
		t.Fatal("分类私密时，即使站点显式公开也不应对匿名可见")
	}
	if !names["无分类站点"] {
		t.Fatal("无分类的继承站点应视为公开")
	}

	var cats categoryListResponse
	e.json(t, http.MethodGet, "/api/categories", nil, &cats)
	for _, c := range cats.Items {
		if c.Name == "内部" {
			t.Fatalf("私密分类不应出现在匿名分类列表里：%+v", c)
		}
	}

	// 历史行的字段本身保留（不静默改数据），只是不再对外可见。
	var dbSite model.Site
	if err := e.st.DB().Where("name = ?", "历史公开站点").First(&dbSite).Error; err != nil {
		t.Fatal(err)
	}
	if dbSite.Visibility != model.VisibilityPublic {
		t.Fatalf("历史行的 visibility 字段应保持 public，得到 %q", dbSite.Visibility)
	}
}

func TestRejectPublicInPrivateCategory(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	privateCat := &model.Category{Name: "私密分类", Visibility: model.VisibilityPrivate}
	if err := e.st.CreateCategory(privateCat); err != nil {
		t.Fatal(err)
	}
	inPrivate := &model.Site{Name: "私密分类站点", URL: "https://p.example.com", CategoryID: &privateCat.ID, Visibility: model.VisibilityInherit}
	if err := e.st.CreateSite(inPrivate); err != nil {
		t.Fatal(err)
	}

	// ① 在私密分类里创建「公开」站点 → 400
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{
		"name": "新公开站点", "url": "https://n.example.com",
		"category_id": privateCat.ID, "visibility": "public",
	}, nil); resp.status != http.StatusBadRequest || !strings.Contains(string(resp.body), "public_in_private_category") {
		t.Fatalf("私密分类里创建公开站点应 400 public_in_private_category，得到 %d %s", resp.status, string(resp.body))
	}

	// ② 私密分类里的站点改成「公开」 → 400；改成「私密」 → 200
	if resp := e.json(t, http.MethodPut, "/api/admin/sites/"+itoa(inPrivate.ID), map[string]any{"visibility": "public"}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("私密分类站点改为公开应 400，得到 %d", resp.status)
	}
	if resp := e.json(t, http.MethodPut, "/api/admin/sites/"+itoa(inPrivate.ID), map[string]any{"visibility": "private"}, nil); resp.status != http.StatusOK {
		t.Fatalf("改为私密应 200，得到 %d", resp.status)
	}

	// ③ 批量设为公开 → 400
	if resp := e.json(t, http.MethodPost, "/api/admin/sites/batch", map[string]any{
		"ids": []uint{inPrivate.ID}, "action": "visibility", "visibility": "public",
	}, nil); resp.status != http.StatusBadRequest || !strings.Contains(string(resp.body), "public_in_private_category") {
		t.Fatalf("批量设为公开应 400 public_in_private_category，得到 %d %s", resp.status, string(resp.body))
	}

	// ④ 批量把显式公开的站点移入私密分类 → 400
	publicSite := &model.Site{Name: "公开站点", URL: "https://o.example.com", Visibility: model.VisibilityPublic}
	if err := e.st.CreateSite(publicSite); err != nil {
		t.Fatal(err)
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/sites/batch", map[string]any{
		"ids": []uint{publicSite.ID}, "action": "category", "category_id": privateCat.ID,
	}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("把公开站点批量移入私密分类应 400，得到 %d %s", resp.status, string(resp.body))
	}

	// ⑤ 分类改为公开后，同一站点再设为公开 → 200
	if resp := e.json(t, http.MethodPut, "/api/admin/categories/"+itoa(privateCat.ID), map[string]any{"visibility": "public"}, nil); resp.status != http.StatusOK {
		t.Fatalf("分类改为公开失败：%d", resp.status)
	}
	if resp := e.json(t, http.MethodPut, "/api/admin/sites/"+itoa(inPrivate.ID), map[string]any{"visibility": "public"}, nil); resp.status != http.StatusOK {
		t.Fatalf("分类公开后站点设为公开应 200，得到 %d", resp.status)
	}
}

func TestAdminCRUDLifecycle(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var created struct {
		Category struct {
			ID uint `json:"id"`
		} `json:"category"`
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/categories", map[string]any{
		"name": "新分类", "visibility": "private",
	}, &created); resp.status != http.StatusOK {
		t.Fatalf("创建分类失败：%d %s", resp.status, string(resp.body))
	}
	var dbCategory model.Category
	if err := e.st.DB().First(&dbCategory, created.Category.ID).Error; err != nil {
		t.Fatalf("分类未落库：%v", err)
	}
	if dbCategory.Name != "新分类" || dbCategory.Visibility != model.VisibilityPrivate {
		t.Fatalf("分类字段不正确：%+v", dbCategory)
	}

	var createdSite struct {
		Site struct {
			ID uint `json:"id"`
		} `json:"site"`
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{
		"name":        "新站点",
		"url":         "https://new.example.com",
		"lan_url":     "http://192.168.1.50:8080",
		"category_id": created.Category.ID,
		"visibility":  "inherit",
		"tags":        "测试",
	}, &createdSite); resp.status != http.StatusOK {
		t.Fatalf("创建站点失败：%d %s", resp.status, string(resp.body))
	}
	var dbSite model.Site
	if err := e.st.DB().First(&dbSite, createdSite.Site.ID).Error; err != nil {
		t.Fatalf("站点未落库：%v", err)
	}
	if dbSite.Name != "新站点" || dbSite.LanURL != "http://192.168.1.50:8080" || dbSite.CategoryID == nil {
		t.Fatalf("站点字段不正确：%+v", dbSite)
	}

	if resp := e.json(t, http.MethodPut, "/api/admin/sites/"+itoa(dbSite.ID), map[string]any{
		"name": "改名站点", "visibility": "public", "pinned": true, "clear_category": true,
	}, nil); resp.status != http.StatusOK {
		t.Fatalf("更新站点失败：%d %s", resp.status, string(resp.body))
	}
	if err := e.st.DB().First(&dbSite, dbSite.ID).Error; err != nil {
		t.Fatal(err)
	}
	if dbSite.Name != "改名站点" || dbSite.Visibility != model.VisibilityPublic || !dbSite.Pinned {
		t.Fatalf("更新未落库：%+v", dbSite)
	}

	// 无效输入必须被拒绝
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{"name": ""}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("空名称应被拒绝，得到 %d", resp.status)
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{"name": "无地址"}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("缺少地址应被拒绝，得到 %d", resp.status)
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{
		"name": "协议错误", "url": "ftp://example.com",
	}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("非 http(s) 协议应被拒绝，得到 %d", resp.status)
	}

	if resp := e.json(t, http.MethodDelete, "/api/admin/sites/"+itoa(dbSite.ID), nil, nil); resp.status != http.StatusOK {
		t.Fatalf("删除站点失败：%d", resp.status)
	}
	var count int64
	if err := e.st.DB().Model(&model.Site{}).Where("id = ?", dbSite.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("站点应从数据库中删除")
	}

	if resp := e.json(t, http.MethodDelete, "/api/admin/categories/"+itoa(created.Category.ID), nil, nil); resp.status != http.StatusOK {
		t.Fatalf("删除分类失败：%d", resp.status)
	}

	// 登出后后台接口不可再用
	if resp := e.do(t, http.MethodPost, "/api/auth/logout", nil); resp.status != http.StatusOK {
		t.Fatalf("登出失败：%d", resp.status)
	}
	if resp := e.do(t, http.MethodGet, "/api/admin/sites", nil); resp.status != http.StatusUnauthorized {
		t.Fatalf("登出后后台接口应 401，得到 %d", resp.status)
	}
	if resp := e.do(t, http.MethodGet, "/api/sites", nil); resp.status != http.StatusUnauthorized {
		t.Fatalf("登出后 private 模式的站点列表应 401，得到 %d", resp.status)
	}
}

func TestSettingsPersistenceAndModeSwitch(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var settings struct {
		AuthMode   string `json:"auth_mode"`
		LANCIDRs   string `json:"lan_cidrs_raw"`
		SiteTitle  string `json:"site_title"`
		Configured bool   `json:"password_configured"`
	}
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"auth_mode":  "private",
		"lan_cidrs":  "192.168.1.0/24,10.0.",
		"site_title": "我的 NAS",
	}, &settings); resp.status != http.StatusOK {
		t.Fatalf("保存设置失败：%d %s", resp.status, string(resp.body))
	}
	if settings.AuthMode != "private" || settings.SiteTitle != "我的 NAS" || !settings.Configured {
		t.Fatalf("设置返回不正确：%+v", settings)
	}

	var cfg struct {
		SiteTitle string   `json:"site_title"`
		AuthMode  string   `json:"auth_mode"`
		LANCIDRs  []string `json:"lan_cidrs"`
	}
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if cfg.SiteTitle != "我的 NAS" || cfg.AuthMode != "private" {
		t.Fatalf("配置未生效：%+v", cfg)
	}
	if len(cfg.LANCIDRs) != 2 || cfg.LANCIDRs[0] != "192.168.1.0/24" {
		t.Fatalf("内网网段解析错误：%+v", cfg.LANCIDRs)
	}

	// 通过 mock 重启（重新打开数据库并新建 server）验证持久化
	srv2, err := server.New(e.cfg, e.st)
	if err != nil {
		t.Fatal(err)
	}
	ts2 := httptest.NewServer(srv2.Handler())
	defer ts2.Close()
	jar, _ := cookiejar.New(nil)
	client2 := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	resp, err := client2.Get(ts2.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), `"auth_mode":"private"`) || !strings.Contains(string(body), "我的 NAS") {
		t.Fatalf("重启后设置应保持：%s", string(body))
	}

	// 非法 auth_mode 应被拒绝
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{"auth_mode": "weird"}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("非法模式应被拒绝，得到 %d", resp.status)
	}
}

func TestSwitchToPrivateRequiresPassword(t *testing.T) {
	e := newEnv(t, "", model.AuthModePublic)

	// 完全没有密码时，后台整体不可用：切模式的接口也拿不到
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{"auth_mode": "private"}, nil); resp.status != http.StatusForbidden {
		t.Fatalf("未设置密码时后台应 403，得到 %d", resp.status)
	}

	// 通过数据库散列设置密码（等价于在后台设置过密码）
	hash, err := auth.HashPassword("db-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetPasswordHash(hash); err != nil {
		t.Fatal(err)
	}
	if resp := e.login(t, "db-password"); resp.status != http.StatusOK {
		t.Fatalf("使用数据库密码登录应成功，得到 %d", resp.status)
	}
	if resp := e.login(t, "wrong-password"); resp.status != http.StatusUnauthorized {
		t.Fatalf("错误密码应 401，得到 %d", resp.status)
	}

	// 已有密码时才允许切换到 private，并可再切回 public
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{"auth_mode": "private"}, nil); resp.status != http.StatusOK {
		t.Fatalf("已设置密码时应允许切换为 private，得到 %d", resp.status)
	}
	var cfg struct {
		AuthMode string `json:"auth_mode"`
	}
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if cfg.AuthMode != "private" {
		t.Fatalf("模式应已切换：%q", cfg.AuthMode)
	}
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{"auth_mode": "public"}, nil); resp.status != http.StatusOK {
		t.Fatalf("切回 public 应成功，得到 %d", resp.status)
	}

	// 密码被移除后（例如不再提供 NASVIA_PASSWORD），后台重新变为不可访问，
	// 前台配置仍可读取，符合「未设置密码仅限内网浏览」的设计。
	if err := e.st.SetPasswordHash(""); err != nil {
		t.Fatal(err)
	}
	if resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]any{"auth_mode": "private"}, nil); resp.status != http.StatusForbidden {
		t.Fatalf("密码被移除后后台应重新拒绝，得到 %d", resp.status)
	}
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if cfg.AuthMode != "public" {
		t.Fatalf("公开模式下前台配置应可读，得到 %q", cfg.AuthMode)
	}
}

func TestSortingChangesOrder(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var created struct {
		Category struct {
			ID uint `json:"id"`
		} `json:"category"`
	}
	e.json(t, http.MethodPost, "/api/admin/categories", map[string]any{"name": "排序测试"}, &created)
	second := created.Category.ID

	if resp := e.json(t, http.MethodPost, "/api/admin/categories/"+itoa(second)+"/move", map[string]any{"direction": "up"}, nil); resp.status != http.StatusOK {
		t.Fatalf("分类上移失败：%d", resp.status)
	}
	var cats categoryListResponse
	e.json(t, http.MethodGet, "/api/admin/categories", nil, &cats)
	index := -1
	for i, c := range cats.Items {
		if c.ID == second {
			index = i
		}
	}
	if index != cats.Total-2 {
		t.Fatalf("分类上移后应位于倒数第二，实际索引 %d / 总数 %d", index, cats.Total)
	}

	var before siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &before)
	if before.Total < 3 {
		t.Skip("站点数量不足，跳过排序断言")
	}
	target := before.Items[len(before.Items)-1].ID
	if resp := e.json(t, http.MethodPost, "/api/admin/sites/"+itoa(target)+"/move", map[string]any{"direction": "up"}, nil); resp.status != http.StatusOK {
		t.Fatalf("站点上移失败：%d", resp.status)
	}
	var after siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &after)
	if after.Items[len(after.Items)-2].ID != target {
		t.Fatalf("站点上移后顺序不正确：%+v", after.Items[len(after.Items)-2])
	}
}

func TestReorderSitesEndpoint(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/reorder", map[string]any{"ids": []uint{1}}, nil); resp.status != http.StatusUnauthorized {
		t.Fatalf("未登录重排应返回 401，得到 %d", resp.status)
	}
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var before siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &before)
	if before.Total < 3 {
		t.Skip("站点数量不足，跳过重排断言")
	}
	ids := make([]uint, 0, len(before.Items))
	for _, item := range before.Items {
		ids = append(ids, item.ID)
	}
	reversed := make([]uint, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		reversed = append(reversed, ids[i])
	}

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/reorder", map[string]any{"ids": reversed}, nil); resp.status != http.StatusOK {
		t.Fatalf("整体重排失败：%d（%s）", resp.status, string(resp.body))
	}
	var after siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &after)
	for i, item := range after.Items {
		if item.ID != reversed[i] {
			t.Fatalf("第 %d 个站点顺序不符：期望 %d 得到 %d", i, reversed[i], item.ID)
		}
	}
	var sorts []int
	if err := e.st.DB().Model(&model.Site{}).Order("sort asc").Pluck("sort", &sorts).Error; err != nil {
		t.Fatal(err)
	}
	for i, value := range sorts {
		if value != i+1 {
			t.Fatalf("sort 未重排为 1..N：第 %d 项 = %d", i, value)
		}
	}

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/reorder", map[string]any{"ids": ids[:len(ids)-1]}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("漏项应返回 400，得到 %d", resp.status)
	}
	dup := append([]uint{ids[0]}, ids...)
	if resp := e.json(t, http.MethodPost, "/api/admin/sites/reorder", map[string]any{"ids": dup}, nil); resp.status != http.StatusBadRequest {
		t.Fatalf("重复项应返回 400，得到 %d", resp.status)
	}
}

func TestBatchOperationsAndPurge(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePrivate)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var list siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &list)
	ids := []uint{list.Items[0].ID, list.Items[1].ID}

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/batch", map[string]any{
		"ids": ids, "action": "visibility", "visibility": "private",
	}, nil); resp.status != http.StatusOK {
		t.Fatalf("批量设置可见性失败：%d", resp.status)
	}
	var count int64
	if err := e.st.DB().Model(&model.Site{}).Where("id IN ? AND visibility = ?", ids, model.VisibilityPrivate).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != int64(len(ids)) {
		t.Fatalf("批量设置未落库：%d", count)
	}

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/batch", map[string]any{"ids": ids, "action": "delete"}, nil); resp.status != http.StatusOK {
		t.Fatalf("批量删除失败：%d", resp.status)
	}
	if err := e.st.DB().Model(&model.Site{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("批量删除未生效")
	}

	if resp := e.json(t, http.MethodPost, "/api/admin/sites/purge", nil, nil); resp.status != http.StatusOK {
		t.Fatalf("清空站点失败：%d", resp.status)
	}
	var after siteListResponse
	e.json(t, http.MethodGet, "/api/sites", nil, &after)
	if after.Total != 0 {
		t.Fatalf("清空后站点应为空，得到 %d", after.Total)
	}

	// 清空后新建站点 + “重启”仍能读到（持久化）
	var created struct {
		Site struct {
			ID uint `json:"id"`
		} `json:"site"`
	}
	e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{"name": "留存站点", "url": "https://keep.example.com"}, &created)

	srv2, err := server.New(e.cfg, e.st)
	if err != nil {
		t.Fatal(err)
	}
	ts2 := httptest.NewServer(srv2.Handler())
	defer ts2.Close()
	jar, _ := cookiejar.New(nil)
	client2 := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	if resp, err := client2.Post(ts2.URL+"/api/auth/login", "application/json", strings.NewReader(`{"password":"test123"}`)); err == nil {
		_ = resp.Body.Close()
	}
	resp, err := client2.Get(ts2.URL + "/api/sites")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "留存站点") {
		t.Fatalf("重启后新站点应仍存在：%s", string(body))
	}
	if strings.Contains(string(body), "Jellyfin") {
		t.Fatal("清空后重启不应复活演示数据")
	}
}

func TestFaviconEndpointCachesAndFallsBack(t *testing.T) {
	// 本地假的“站点”：提供 favicon.ico
	icons := http.NewServeMux()
	icons.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	})
	iconServer := httptest.NewServer(icons)
	defer iconServer.Close()

	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}

	var created struct {
		Site struct {
			ID uint `json:"id"`
		} `json:"site"`
	}
	if resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{
		"name": "带图标站点", "url": iconServer.URL, "visibility": "public",
	}, &created); resp.status != http.StatusOK {
		t.Fatalf("创建站点失败：%d", resp.status)
	}

	// 抓取是异步的：轮询等待图标落库
	deadline := time.Now().Add(10 * time.Second)
	var site model.Site
	for time.Now().Before(deadline) {
		if err := e.st.DB().First(&site, created.Site.ID).Error; err == nil && site.IconHash != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if site.IconHash == "" {
		t.Fatalf("图标应在后台抓取完成并落库（icon_state=%s）", site.IconState)
	}

	entries, err := os.ReadDir(filepath.Join(e.dir, "icons"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("图标缓存文件应存在：err=%v count=%d", err, len(entries))
	}

	iconResp := e.do(t, http.MethodGet, "/api/sites/"+itoa(site.ID)+"/icon", nil)
	if iconResp.status != http.StatusOK || !strings.HasPrefix(iconResp.header.Get("Content-Type"), "image/") {
		t.Fatalf("站点图标应返回图片：%d %s", iconResp.status, iconResp.header.Get("Content-Type"))
	}

	// 图标响应一律带沙箱 CSP：远程抓来的 SVG 以本站同源输出，不能让它执行脚本
	if csp := iconResp.header.Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Fatalf("图标响应应带沙箱 CSP，得到 %q", csp)
	}
	if cc := iconResp.header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatalf("不带版本号的图标地址不能长期缓存，得到 %q", cc)
	}
	// 带当前内容版本（?v=哈希前 8 位）的地址可以长期缓存：图标一变地址就变
	versioned := e.do(t, http.MethodGet, "/api/sites/"+itoa(site.ID)+"/icon?v="+site.IconHash[:8], nil)
	if cc := versioned.header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("带版本号的图标应长期缓存，得到 %q", cc)
	}

	// 按任意域名抓图标的接口已移除（前端不用，且会被当作同源 SVG 注入的跳板）
	if resp := e.do(t, http.MethodGet, "/api/favicon?domain=not-exist.invalid", nil); resp.status != http.StatusNotFound {
		t.Fatalf("/api/favicon 应已移除，得到 %d", resp.status)
	}

	// 修改地址后应重置图标状态并重新排队抓取
	if resp := e.json(t, http.MethodPut, "/api/admin/sites/"+itoa(site.ID), map[string]any{
		"name": "带图标站点", "url": "https://cached.example.com",
	}, nil); resp.status != http.StatusOK {
		t.Fatalf("更新站点失败：%d", resp.status)
	}
	var updated model.Site
	if err := e.st.DB().First(&updated, site.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.IconHash != "" {
		t.Fatalf("地址变更后应清空已缓存图标，得到 %q", updated.IconHash)
	}
	if updated.IconState != model.IconStatePending && updated.IconState != model.IconStateReady && updated.IconState != model.IconStateFailed {
		t.Fatalf("地址变更后图标状态应重置，得到 %q", updated.IconState)
	}
}

func TestSiteCreationIsNotBlockedByIconFetch(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatal("登录失败")
	}
	for i := 0; i < 10; i++ {
		start := time.Now()
		resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{
			"name": "并发站点" + itoa(uint(i)),
			"url":  "https://slow-" + itoa(uint(i)) + ".example.com",
		}, nil)
		elapsed := time.Since(start)
		if resp.status != http.StatusOK {
			t.Fatalf("创建站点失败：%d", resp.status)
		}
		if elapsed > 800*time.Millisecond {
			t.Fatalf("创建站点被图标抓取阻塞：耗时 %v", elapsed)
		}
	}
}

func TestStaticAssetsAndIndexPage(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)

	index := e.do(t, http.MethodGet, "/", nil)
	if index.status != http.StatusOK {
		t.Fatalf("首页状态码 %d", index.status)
	}
	if ct := index.header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("首页 Content-Type 应为 text/html，得到 %s", ct)
	}
	html := string(index.body)
	if !strings.Contains(html, "<title>") || !strings.Contains(html, `<div id="root">`) {
		t.Fatalf("首页应包含标题与挂载点：%s", html[:min(len(html), 200)])
	}
	if !strings.Contains(html, "/assets/") {
		t.Fatal("首页应引用 /assets/ 下的构建产物")
	}

	assetPath := extractAssetPath(html)
	if assetPath == "" {
		t.Fatal("未能从首页解析出资源路径")
	}
	asset := e.do(t, http.MethodGet, assetPath, nil)
	if asset.status != http.StatusOK {
		t.Fatalf("资源 %s 状态码 %d", assetPath, asset.status)
	}
	if ct := asset.header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("JS 资源 Content-Type 错误：%s", ct)
	}
	if cache := asset.header.Get("Cache-Control"); !strings.Contains(cache, "max-age") {
		t.Fatalf("带哈希的资源应带长缓存头，得到 %q", cache)
	}
	if len(asset.body) == 0 {
		t.Fatal("资源内容为空")
	}

	// 未知资源不应回落成 HTML
	missing := e.do(t, http.MethodGet, "/assets/does-not-exist.js", nil)
	if missing.status != http.StatusNotFound {
		t.Fatalf("未知资源应 404，得到 %d", missing.status)
	}

	// 前端路由回落
	spa := e.do(t, http.MethodGet, "/admin", nil)
	if spa.status != http.StatusOK || !strings.Contains(string(spa.body), `<div id="root">`) {
		t.Fatalf("SPA 路由应回落 index.html，得到 %d", spa.status)
	}
}

func TestHealthAndConfig(t *testing.T) {
	e := newEnv(t, "", model.AuthModePublic)

	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if resp := e.json(t, http.MethodGet, "/api/health", nil, &health); resp.status != http.StatusOK || health.Status != "ok" {
		t.Fatalf("健康检查失败：%+v", health)
	}

	var cfg struct {
		SiteTitle          string `json:"site_title"`
		AuthMode           string `json:"auth_mode"`
		Authenticated      bool   `json:"authenticated"`
		PasswordConfigured bool   `json:"password_configured"`
		Version            string `json:"version"`
	}
	if resp := e.json(t, http.MethodGet, "/api/config", nil, &cfg); resp.status != http.StatusOK {
		t.Fatalf("config 接口失败：%d", resp.status)
	}
	if cfg.Authenticated || cfg.PasswordConfigured {
		t.Fatalf("匿名配置不应显示已登录/已设密码：%+v", cfg)
	}
	if cfg.SiteTitle == "" || cfg.Version == "" {
		t.Fatalf("配置缺少必要字段：%+v", cfg)
	}
}

func extractAssetPath(html string) string {
	const marker = `src="`
	idx := strings.Index(html, marker+"/assets/")
	if idx < 0 {
		return ""
	}
	rest := html[idx+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var buf []byte
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}

// TestAdminIconName 后台手动指定图标：解析、校验、清空，以及匿名响应不泄露这两个诊断字段。
func TestAdminIconName(t *testing.T) {
	e := newEnv(t, "secret", model.AuthModePublic)
	// 本地索引缓存（只用于匹配与校验，不会触发任何外网请求）
	index := &hdicons.Index{Name: "test", Count: 2, Items: []hdicons.Item{
		{Name: "baidu-drive-1", URL: "https://example.invalid/border-radius/baidu-drive-1.png"},
		{Name: "synology-photo-1", URL: "https://example.invalid/border-radius/synology-photo-1.png"},
	}}
	if err := hdicons.WriteCache(filepath.Join(e.dir, "hdicons.json"), index); err != nil {
		t.Fatalf("write index cache: %v", err)
	}

	// 匿名响应不得出现 icon_name / icon_auto
	anon := e.do(t, http.MethodGet, "/api/sites", nil)
	if anon.status != http.StatusOK {
		t.Fatalf("匿名站点列表状态 %d", anon.status)
	}
	for _, field := range []string{"icon_name", "icon_auto"} {
		if bytes.Contains(anon.body, []byte(field)) {
			t.Errorf("匿名响应不应包含 %s：%s", field, string(anon.body))
		}
	}

	e.login(t, "secret")
	list := siteListResponse{}
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &list)
	if len(list.Items) == 0 {
		t.Fatal("后台站点列表为空")
	}
	path := "/api/admin/sites/" + strconv.FormatUint(uint64(list.Items[0].ID), 10)

	// 手动名支持「前缀 + 变体号」写法，并被归一成真实条目名
	updated := struct {
		Site struct {
			IconName  string `json:"icon_name"`
			IconState string `json:"icon_state"`
		} `json:"site"`
	}{}
	if resp := e.json(t, http.MethodPut, path, map[string]any{"icon_name": "baidu-drive"}, &updated); resp.status != http.StatusOK {
		t.Fatalf("设置 icon_name 状态 %d：%s", resp.status, string(resp.body))
	}
	if updated.Site.IconName != "baidu-drive-1" {
		t.Errorf("icon_name 应归一成 baidu-drive-1，得到 %q", updated.Site.IconName)
	}
	if updated.Site.IconState != model.IconStatePending {
		t.Errorf("改图标名后应重置为 pending，得到 %q", updated.Site.IconState)
	}

	// 索引里没有的名字：400
	if resp := e.do(t, http.MethodPut, path, map[string]any{"icon_name": "不存在的图标"}); resp.status != http.StatusBadRequest {
		t.Errorf("未知图标名应 400，得到 %d：%s", resp.status, string(resp.body))
	}

	// 清空后回到自动匹配
	cleared := struct {
		Site struct {
			IconName string `json:"icon_name"`
		} `json:"site"`
	}{}
	e.json(t, http.MethodPut, path, map[string]any{"clear_icon_name": true}, &cleared)
	if cleared.Site.IconName != "" {
		t.Errorf("清空后 icon_name 应为空，得到 %q", cleared.Site.IconName)
	}

	// 图标搜索：英文与中文都能找到（中文走关键词/别名表）
	for _, query := range []string{"baidu", "百度", "网盘"} {
		var found struct {
			Items []string `json:"items"`
		}
		e.json(t, http.MethodGet, "/api/admin/hdicons/search?q="+url.QueryEscape(query), nil, &found)
		if len(found.Items) == 0 || found.Items[0] != "baidu-drive-1" {
			t.Errorf("搜索 %q 应命中 baidu-drive-1，得到 %v", query, found.Items)
		}
	}

	// 预览接口：未知名字 404（不发外网请求）
	if resp := e.do(t, http.MethodGet, "/api/admin/hdicons/icon?name=nope", nil); resp.status != http.StatusNotFound {
		t.Errorf("未知图标预览应 404，得到 %d", resp.status)
	}

	// 未登录不得访问后台接口
	_ = e.do(t, http.MethodPost, "/api/auth/logout", nil)
	if resp := e.do(t, http.MethodGet, "/api/admin/hdicons/search?q=baidu", nil); resp.status != http.StatusUnauthorized {
		t.Errorf("未登录访问图标搜索应 401，得到 %d", resp.status)
	}
}

// TestIconSourceFallback 图标来源回退顺序：HD-Icons 命中就不碰 nasicon；HD-Icons 没有才用 nasicon；
// 两个来源都没有则落占位图。全程用本地 stub 扮演两个来源，不依赖外网。
func TestIconSourceFallback(t *testing.T) {
	hdPayload := []byte("\x89PNG\r\n\x1a\nHDICONS-PAYLOAD")
	nasPayload := []byte("\x89PNG\r\n\x1a\nNASICON-PAYLOAD")

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hdicons/icons.json":
			_, _ = io.WriteString(w, `{"name":"stub","total_count":1,"icons":[{"name":"jellyfin-1","url":"/border-radius/jellyfin-1.png"}]}`)
		case "/hdicons/border-radius/jellyfin-1.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(hdPayload)
		case "/nasicon/icons.json":
			// nasicon 里也有 Jellyfin（用来验证 HD-Icons 优先），另有一条中文名条目
			_, _ = io.WriteString(w, `[{"name":"Jellyfin","filename":"Jellyfin.png"},`+
				`{"name":"Baidunetdisk A","cnName":"百度网盘","domain":"pan.baidu.com","filename":"Baidunetdisk_A--百度网盘--qdnas-s.png"}]`)
		case "/nasicon/icon/Jellyfin.png", "/nasicon/icon/Baidunetdisk_A--百度网盘--qdnas-s.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(nasPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer stub.Close()

	e := newEnvWith(t, "secret", model.AuthModePublic, func(cfg *config.Config) {
		// 只留这两个来源：既验证优先级，也避免 favicon 链路真的去访问外网
		cfg.FaviconSources = "hdicons,nasicon"
		cfg.HDIconsMirrors = stub.URL + "/hdicons"
		cfg.NASIconBase = stub.URL + "/nasicon"
	})
	e.login(t, "secret")

	createSite := func(name, url string) uint {
		t.Helper()
		var out struct {
			Site struct {
				ID uint `json:"id"`
			} `json:"site"`
		}
		resp := e.json(t, http.MethodPost, "/api/admin/sites", map[string]any{"name": name, "url": url}, &out)
		if resp.status != http.StatusOK || out.Site.ID == 0 {
			t.Fatalf("创建站点 %q 失败：%d %s", name, resp.status, string(resp.body))
		}
		return out.Site.ID
	}
	waitSource := func(id uint, want string) string {
		t.Helper()
		got := ""
		for i := 0; i < 100; i++ {
			if site, err := e.st.Site(id); err == nil {
				got = site.IconSource
				if got == want {
					return got
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		return got
	}
	iconBytes := func(id uint) []byte {
		t.Helper()
		return e.do(t, http.MethodGet, "/api/sites/"+strconv.FormatUint(uint64(id), 10)+"/icon", nil).body
	}

	// 1) HD-Icons 命中 → 用 HD-Icons 的字节（nasicon 里也有同名条目，但轮不到它）
	hdID := createSite("Jellyfin", "https://jellyfin.home.example.com")
	if got := waitSource(hdID, "hdicons:jellyfin-1"); got != "hdicons:jellyfin-1" {
		t.Errorf("HD-Icons 应优先命中，icon_source = %q", got)
	}
	if body := iconBytes(hdID); !bytes.Equal(body, hdPayload) {
		t.Errorf("应从 HD-Icons 取图，得到 %q", string(body))
	}

	// 2) HD-Icons 没有 → nasicon 按中文名命中
	nasID := createSite("百度网盘", "https://pan.baidu.com/")
	wantNAS := "nasicon:Baidunetdisk_A--百度网盘--qdnas-s.png"
	if got := waitSource(nasID, wantNAS); got != wantNAS {
		t.Errorf("HD-Icons 未命中时应由 nasicon 兜底，icon_source = %q", got)
	}
	if body := iconBytes(nasID); !bytes.Equal(body, nasPayload) {
		t.Errorf("应从 nasicon 取图，得到 %q", string(body))
	}
	// 对外归一的档位是 nasicon（前端据此不套灰底）
	var list siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &list)
	for _, item := range list.Items {
		if item.ID == nasID && item.IconSource != model.IconSourceNASIcon {
			t.Errorf("nasicon 图标的 icon_source 档位应为 %q，得到 %q", model.IconSourceNASIcon, item.IconSource)
		}
	}

	// 3) 两个来源都没有 → 占位图（不 5xx）
	noneID := createSite("未知服务", "https://unknown.example.com")
	if got := waitSource(noneID, model.IconSourcePlaceholder); got != model.IconSourcePlaceholder {
		t.Errorf("两个来源都匹配不到时应落占位图，icon_source = %q", got)
	}
	if resp := e.do(t, http.MethodGet, "/api/sites/"+strconv.FormatUint(uint64(noneID), 10)+"/icon", nil); resp.status != http.StatusOK {
		t.Errorf("占位图应返回 200，得到 %d", resp.status)
	}

	// 4) 后台诊断字段能显示两个来源的匹配结果
	for _, item := range list.Items {
		if item.ID == hdID && item.IconAuto != "jellyfin-1" {
			t.Errorf("Jellyfin 的 icon_auto 应为 jellyfin-1，得到 %q", item.IconAuto)
		}
	}
	var after siteListResponse
	e.json(t, http.MethodGet, "/api/admin/sites", nil, &after)
	found := false
	for _, item := range after.Items {
		if item.ID == nasID {
			found = true
			if item.IconAuto != "nasicon:Baidunetdisk_A--百度网盘--qdnas-s.png" {
				t.Errorf("百度网盘的 icon_auto 应显示 nasicon 匹配结果，得到 %q", item.IconAuto)
			}
		}
	}
	if !found {
		t.Error("后台列表里应有百度网盘")
	}
}

type categoryIconResponse struct {
	Category struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Icon string `json:"icon"`
	} `json:"category"`
}

type frontCategoryResponse struct {
	Items []struct {
		ID   uint   `json:"id"`
		Icon string `json:"icon"`
	} `json:"items"`
}

type apiCodeResponse struct {
	Code string `json:"code"`
}

// TestCategoryIconRoundTrip 分类图标键：创建/更新/清空往返落库，非法键 400 invalid_icon。
func TestCategoryIconRoundTrip(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("登录应成功，得到 %d", resp.status)
	}

	var created categoryIconResponse
	resp := e.json(t, http.MethodPost, "/api/admin/categories", map[string]string{"name": "影音", "icon": "film"}, &created)
	if resp.status != http.StatusOK {
		t.Fatalf("创建带图标的分类应成功，得到 %d（%s）", resp.status, string(resp.body))
	}
	if created.Category.Icon != "film" {
		t.Fatalf("创建响应里的 icon 应为 film，得到 %q", created.Category.Icon)
	}
	id := strconv.Itoa(int(created.Category.ID))

	var stored model.Category
	if err := e.st.DB().First(&stored, created.Category.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Icon != "film" {
		t.Fatalf("图标应落库为 film，得到 %q", stored.Icon)
	}

	var updated categoryIconResponse
	e.json(t, http.MethodPut, "/api/admin/categories/"+id, map[string]string{"icon": "shield"}, &updated)
	if updated.Category.Icon != "shield" {
		t.Fatalf("更新后 icon 应为 shield，得到 %q", updated.Category.Icon)
	}

	// 清空（空串）= 回到「按分类名自动匹配」
	var cleared categoryIconResponse
	e.json(t, http.MethodPut, "/api/admin/categories/"+id, map[string]string{"icon": ""}, &cleared)
	if cleared.Category.Icon != "" {
		t.Fatalf("空串应清空图标，得到 %q", cleared.Category.Icon)
	}

	// 非法图标键：创建与更新都要 400，且更新失败不能落库
	var apiErr apiCodeResponse
	resp = e.json(t, http.MethodPost, "/api/admin/categories", map[string]string{"name": "非法图标", "icon": "Bad_Icon!"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_icon" {
		t.Fatalf("非法图标键应 400 invalid_icon，得到 %d %s", resp.status, string(resp.body))
	}
	resp = e.json(t, http.MethodPut, "/api/admin/categories/"+id, map[string]string{"icon": "带中文"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_icon" {
		t.Fatalf("更新为非法图标键应 400 invalid_icon，得到 %d %s", resp.status, string(resp.body))
	}
	if err := e.st.DB().First(&stored, created.Category.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Icon != "" {
		t.Fatalf("非法图标不应落库，得到 %q", stored.Icon)
	}

	// 前台分类列表也要带 icon（导航渲染用）
	var front frontCategoryResponse
	e.json(t, http.MethodGet, "/api/categories", nil, &front)
	found := false
	for _, item := range front.Items {
		if item.ID == created.Category.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("前台分类列表里应有刚创建的分类")
	}
}

type navSettingsResponse struct {
	NavPosition string `json:"nav_position"`
	NavStyle    string `json:"nav_style"`
	NavVisible  string `json:"nav_visible"`
}

// TestNavSettingsAndConfigEcho 导航位置/样式：默认值、合法值落库与回显、非法值 400 且不落库。
func TestNavSettingsAndConfigEcho(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)

	var cfg navSettingsResponse
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if cfg.NavPosition != model.NavPositionLeft || cfg.NavStyle != model.NavStyleIcon || cfg.NavVisible != model.NavVisibleShow {
		t.Fatalf("默认导航设置应为 left/icon/show（默认仅图标），得到 %q/%q/%q", cfg.NavPosition, cfg.NavStyle, cfg.NavVisible)
	}

	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("登录应成功，得到 %d", resp.status)
	}

	var settings navSettingsResponse
	resp := e.json(t, http.MethodPut, "/api/admin/settings",
		map[string]string{"nav_position": "left", "nav_style": "icon", "nav_visible": "hide"}, &settings)
	if resp.status != http.StatusOK || settings.NavPosition != "left" || settings.NavStyle != "icon" || settings.NavVisible != "hide" {
		t.Fatalf("保存左侧+仅图标+隐藏导航应成功并回显，得到 %d %s", resp.status, string(resp.body))
	}

	// 匿名访客从 /api/config 读到同一套设置（后台统一）
	cfg = navSettingsResponse{}
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if cfg.NavPosition != "left" || cfg.NavStyle != "icon" || cfg.NavVisible != "hide" {
		t.Fatalf("/api/config 应回显 left/icon/hide，得到 %q/%q/%q", cfg.NavPosition, cfg.NavStyle, cfg.NavVisible)
	}

	var apiErr apiCodeResponse
	resp = e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{"nav_position": "middle"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_nav_position" {
		t.Fatalf("非法 nav_position 应 400，得到 %d %s", resp.status, string(resp.body))
	}
	resp = e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{"nav_style": "emoji"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_nav_style" {
		t.Fatalf("非法 nav_style 应 400，得到 %d %s", resp.status, string(resp.body))
	}

	resp = e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{"nav_visible": "maybe"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_nav_visible" {
		t.Fatalf("非法 nav_visible 应 400，得到 %d %s", resp.status, string(resp.body))
	}

	settings = navSettingsResponse{}
	e.json(t, http.MethodGet, "/api/admin/settings", nil, &settings)
	if settings.NavPosition != "left" || settings.NavStyle != "icon" || settings.NavVisible != "hide" {
		t.Fatalf("非法请求不应改动已保存的设置，得到 %q/%q/%q", settings.NavPosition, settings.NavStyle, settings.NavVisible)
	}

	// 已取消的顶部导航：接口必须拒绝，且不改动已保存的值
	resp = e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{"nav_position": "top"}, &apiErr)
	if resp.status != http.StatusBadRequest || apiErr.Code != "invalid_nav_position" {
		t.Fatalf("已取消的 nav_position=top 应 400 invalid_nav_position，得到 %d %s", resp.status, string(resp.body))
	}
	settings = navSettingsResponse{}
	e.json(t, http.MethodGet, "/api/admin/settings", nil, &settings)
	if settings.NavPosition != "left" {
		t.Fatalf("非法请求不应改动已保存的位置，得到 %q", settings.NavPosition)
	}

	// 历史库里可能存着 top：读取（含 /api/config）必须按 left 渲染
	if err := e.st.SetSetting(model.SettingNavPosition, "top"); err != nil {
		t.Fatal(err)
	}
	settings = navSettingsResponse{}
	e.json(t, http.MethodGet, "/api/admin/settings", nil, &settings)
	cfg = navSettingsResponse{}
	e.json(t, http.MethodGet, "/api/config", nil, &cfg)
	if settings.NavPosition != "left" || cfg.NavPosition != "left" {
		t.Fatalf("历史值 top 应按 left 返回，得到 settings=%q config=%q", settings.NavPosition, cfg.NavPosition)
	}

	// 空串 = 回落默认值（left/text）
	settings = navSettingsResponse{}
	e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{"nav_position": "", "nav_style": "", "nav_visible": ""}, &settings)
	if settings.NavPosition != model.NavPositionLeft || settings.NavStyle != model.NavStyleIcon || settings.NavVisible != model.NavVisibleShow {
		t.Fatalf("空串应回落 left/icon/show，得到 %q/%q/%q", settings.NavPosition, settings.NavStyle, settings.NavVisible)
	}
}

// configNetwork 带自定义请求头取 /api/config 里的内外网判定。
func (e *env) configNetwork(t *testing.T, header map[string]string) (mode, clientIP, reason string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.ts.URL+"/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Network struct {
			Mode     string `json:"mode"`
			ClientIP string `json:"client_ip"`
			Reason   string `json:"reason"`
		} `json:"network"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Network.Mode, out.Network.ClientIP, out.Network.Reason
}

// TestNetworkDetectionBehindProxy 经反代（NPM → WireGuard → NAS）访问同一个域名时，按真实访客 IP 判定内外网。
func TestNetworkDetectionBehindProxy(t *testing.T) {
	e := newEnvWith(t, "pw-123456", model.AuthModePublic, func(cfg *config.Config) {
		cfg.TrustedProxies = config.DefaultTrustedProxies // 测试客户端从 127.0.0.1 连入，视为可信反代
	})
	e.login(t, "pw-123456")

	// 非法条目整体拒绝，不落库
	if resp := e.do(t, http.MethodPut, "/api/admin/settings", map[string]string{"home_egress": "203.0.113.7\nbad_host"}); resp.status != http.StatusBadRequest {
		t.Fatalf("非法家庭出口应返回 400，得到 %d：%s", resp.status, resp.body)
	}
	var saved struct {
		HomeEgress string `json:"home_egress_raw"`
	}
	resp := e.json(t, http.MethodPut, "/api/admin/settings", map[string]string{
		"lan_cidrs":   "192.168.1.0/24",
		"home_egress": "203.0.113.7, https://203.0.113.7/",
	}, &saved)
	if resp.status != http.StatusOK || saved.HomeEgress != "203.0.113.7" {
		t.Fatalf("保存家庭出口失败：%d %s", resp.status, resp.body)
	}

	cases := []struct {
		name, xff, mode, ip, reason string
	}{
		{"在家（家庭公网出口）", "203.0.113.7", "lan", "203.0.113.7", "home_egress"},
		{"在单位", "198.51.100.20", "wan", "198.51.100.20", ""},
		{"家里做了 DNS 分流、局域网直达", "192.168.1.31", "lan", "192.168.1.31", "lan_rule"},
		{"访客伪造的 XFF 在最左、反代追加的真实 IP 在最右", "203.0.113.7, 198.51.100.20", "wan", "198.51.100.20", ""},
	}
	for _, c := range cases {
		mode, ip, reason := e.configNetwork(t, map[string]string{"X-Forwarded-For": c.xff})
		if mode != c.mode || ip != c.ip || reason != c.reason {
			t.Errorf("%s：得到 %s/%s/%s，期望 %s/%s/%s", c.name, mode, ip, reason, c.mode, c.ip, c.reason)
		}
	}
}

// TestUntrustedProxyHeadersIgnored 请求不是从可信反代来的：X-Forwarded-For 一律忽略，按直连 IP 判定。
func TestUntrustedProxyHeadersIgnored(t *testing.T) {
	e := newEnvWith(t, "", model.AuthModePublic, func(cfg *config.Config) {
		cfg.TrustedProxies = "10.8.0.1" // 只信任 WireGuard 对端，127.0.0.1 不在其中
		cfg.HomeEgress = "203.0.113.7"
	})
	mode, ip, _ := e.configNetwork(t, map[string]string{"X-Forwarded-For": "203.0.113.7"})
	if mode != "wan" || ip != "127.0.0.1" {
		t.Fatalf("不可信来源的 XFF 应被忽略：得到 %s/%s", mode, ip)
	}
}

// TestChangePassword 后台改密码（POST /api/admin/password）：弱密码拒绝，改完后旧密码失效、新密码可登录；
// 其他设备的会话立即失效，改密码的这台设备换发新会话、保持登录。
func TestChangePassword(t *testing.T) {
	e := newEnv(t, "old-pass-1", model.AuthModePublic)
	if resp := e.login(t, "old-pass-1"); resp.status != http.StatusOK {
		t.Fatalf("登录失败：%d", resp.status)
	}
	other := *e // 另一台设备：同一服务，独立的 Cookie
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	other.client = &http.Client{Jar: jar, Timeout: 30 * time.Second}
	if resp := other.login(t, "old-pass-1"); resp.status != http.StatusOK {
		t.Fatalf("另一台设备登录失败：%d", resp.status)
	}
	if resp := e.do(t, http.MethodPost, "/api/admin/password", map[string]string{"password": "123"}); resp.status != http.StatusBadRequest {
		t.Fatalf("弱密码应返回 400，得到 %d", resp.status)
	}
	if resp := e.do(t, http.MethodPost, "/api/admin/password", map[string]string{"password": "new-pass-2"}); resp.status != http.StatusOK {
		t.Fatalf("改密码失败：%d %s", resp.status, resp.body)
	}
	if resp := other.do(t, http.MethodGet, "/api/admin/settings", nil); resp.status != http.StatusUnauthorized {
		t.Fatalf("改密码后其他设备的会话应失效，得到 %d", resp.status)
	}
	if resp := e.do(t, http.MethodGet, "/api/admin/settings", nil); resp.status != http.StatusOK {
		t.Fatalf("改密码的这台设备应换发新会话、保持登录，得到 %d", resp.status)
	}
	if resp := e.login(t, "old-pass-1"); resp.status != http.StatusUnauthorized {
		t.Fatalf("旧密码应失效，得到 %d", resp.status)
	}
	if resp := e.login(t, "new-pass-2"); resp.status != http.StatusOK {
		t.Fatalf("新密码应能登录，得到 %d", resp.status)
	}
}

// TestCrossOriginWritesRejected 同根域名下的其它子站（same-site 但不同源）借已登录的 Cookie 调后台写接口必须被拒；
// 同源页面的写请求与 GET 请求照常放行。
func TestCrossOriginWritesRejected(t *testing.T) {
	e := newEnv(t, "test123", model.AuthModePublic)
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("登录失败：%d", resp.status)
	}
	send := func(method, path, body string, header map[string]string) int {
		t.Helper()
		req, err := http.NewRequest(method, e.ts.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "text/plain") // 「简单请求」：浏览器不发预检，CORS 拦不住
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := e.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	sibling := map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://blog.example.com"}
	if got := send(http.MethodPost, "/api/admin/password", `{"password":"hacked-1"}`, sibling); got != http.StatusForbidden {
		t.Fatalf("兄弟子站发起的改密码请求应返回 403，得到 %d", got)
	}
	// 没有 Sec-Fetch-Site 的旧浏览器：按 Origin 与 Host 比对
	if got := send(http.MethodPost, "/api/admin/purge", "", map[string]string{"Origin": "https://blog.example.com"}); got != http.StatusForbidden {
		t.Fatalf("Origin 与 Host 不符的写请求应返回 403，得到 %d", got)
	}
	if got := send(http.MethodGet, "/api/admin/settings", "", sibling); got != http.StatusOK {
		t.Fatalf("GET 不应受影响，得到 %d", got)
	}
	if resp := e.login(t, "test123"); resp.status != http.StatusOK {
		t.Fatalf("被拦下的请求不应改掉密码，原密码登录得到 %d", resp.status)
	}
	if got := send(http.MethodPost, "/api/admin/password", `{"password":"new-pass-2"}`, map[string]string{"Sec-Fetch-Site": "same-origin"}); got != http.StatusOK {
		t.Fatalf("同源页面的写请求应放行，得到 %d", got)
	}
}
