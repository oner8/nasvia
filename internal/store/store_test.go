package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oner8/nasvia/internal/model"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "nasvia.db"), filepath.Join(dir, "icons"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return st
}

func TestSeedOnlyOnce(t *testing.T) {
	st := newStore(t)
	seeded, err := st.Seed()
	if err != nil || !seeded {
		t.Fatalf("首次应播种：seeded=%v err=%v", seeded, err)
	}
	sites, err := st.Sites()
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 50 {
		t.Fatalf("演示站点应为 50 条（5 个分类 × 10），实际 %d", len(sites))
	}
	cats, err := st.Categories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 5 {
		t.Fatalf("演示分类应为 5 个（含 1 个私密分类），实际 %d", len(cats))
	}
	perCategory := map[uint]int{}
	for _, s := range sites {
		if s.CategoryID != nil {
			perCategory[*s.CategoryID]++
		}
	}
	for _, c := range cats {
		if perCategory[c.ID] != 10 {
			t.Fatalf("分类「%s」应有 10 个演示站点，实际 %d", c.Name, perCategory[c.ID])
		}
	}

	privateCount, privateCatCount := 0, 0
	for _, s := range sites {
		if s.Visibility == model.VisibilityPrivate {
			privateCount++
		}
	}
	for _, c := range cats {
		if c.Visibility == model.VisibilityPrivate {
			privateCatCount++
		}
	}
	if privateCount < 3 {
		t.Fatalf("演示数据应包含至少 3 个私密站点，实际 %d", privateCount)
	}
	if privateCatCount != 1 {
		t.Fatalf("演示数据应恰好包含 1 个私密分类，实际 %d", privateCatCount)
	}
	names := map[string]bool{}
	for _, c := range cats {
		names[c.Name] = true
	}
	if !names["私密分类"] {
		t.Fatalf("私密分类的演示名应为「私密分类」，实际分类：%+v", names)
	}
	if names["私密管理"] {
		t.Fatal("旧名「私密管理」不应再出现在演示数据里")
	}

	again, err := st.Seed()
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("已初始化过的实例不应再次播种（清空数据后不能复活演示数据）")
	}
}

func TestCategoryAndSiteCRUD(t *testing.T) {
	st := newStore(t)
	category := &model.Category{Name: "媒体中心", Visibility: model.VisibilityPublic}
	if err := st.CreateCategory(category); err != nil {
		t.Fatal(err)
	}
	if category.Sort == 0 {
		t.Fatal("新建分类应自动分配排序值")
	}

	site := &model.Site{Name: "Jellyfin", URL: "https://jellyfin.example.com", CategoryID: &category.ID}
	if err := st.CreateSite(site); err != nil {
		t.Fatal(err)
	}

	updated, err := st.UpdateSite(site.ID, map[string]any{"name": "Jellyfin 2", "visibility": model.VisibilityPrivate})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Jellyfin 2" || updated.Visibility != model.VisibilityPrivate {
		t.Fatalf("更新未生效：%+v", updated)
	}

	cleared, err := st.UpdateSite(site.ID, map[string]any{"category_id": nil})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.CategoryID != nil {
		t.Fatalf("应能清空分类引用：%+v", cleared.CategoryID)
	}

	if err := st.SetSiteIcon(site.ID, "abc123", "image/png", "hdicons:foo-1"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Site(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IconHash != "abc123" || got.IconState != model.IconStateReady {
		t.Fatalf("图标状态未写入：%+v", got)
	}
	if err := st.MarkIconPending(site.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = st.Site(site.ID)
	if got.IconHash != "" || got.IconState != model.IconStatePending {
		t.Fatalf("MarkIconPending 未生效：%+v", got)
	}

	if err := st.DeleteSite(site.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Site(site.ID); err != ErrNotFound {
		t.Fatalf("删除后应返回 ErrNotFound，得到 %v", err)
	}
}

func TestDeleteCategoryKeepsSites(t *testing.T) {
	st := newStore(t)
	category := &model.Category{Name: "临时"}
	if err := st.CreateCategory(category); err != nil {
		t.Fatal(err)
	}
	site := &model.Site{Name: "A", URL: "https://a.example.com", CategoryID: &category.ID}
	if err := st.CreateSite(site); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteCategory(category.ID); err != nil {
		t.Fatal(err)
	}
	got, err := st.Site(site.ID)
	if err != nil {
		t.Fatalf("站点不应被删除：%v", err)
	}
	if got.CategoryID != nil {
		t.Fatalf("站点应变为未分类：%+v", got.CategoryID)
	}
	if err := st.DeleteCategory(category.ID); err != ErrNotFound {
		t.Fatalf("重复删除应返回 ErrNotFound，得到 %v", err)
	}
}

func TestMoveAndReorder(t *testing.T) {
	st := newStore(t)
	ids := make([]uint, 0, 3)
	for _, name := range []string{"A", "B", "C"} {
		site := &model.Site{Name: name, URL: "https://" + name + ".example.com"}
		if err := st.CreateSite(site); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, site.ID)
	}

	if err := st.MoveSite(ids[2], -1); err != nil {
		t.Fatal(err)
	}
	order := func() []string {
		sites, err := st.Sites()
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(sites))
		for _, s := range sites {
			out = append(out, s.Name)
		}
		return out
	}
	if got := order(); got[1] != "C" || got[2] != "B" {
		t.Fatalf("上移未生效：%v", got)
	}

	if err := st.MoveSite(ids[0], -1); err != nil {
		t.Fatalf("越界移动不应报错：%v", err)
	}
	if got := order(); got[0] != "A" {
		t.Fatalf("越界移动应保持顺序：%v", got)
	}
	if err := st.MoveSite(9999, 1); err != ErrNotFound {
		t.Fatalf("不存在的站点应返回 ErrNotFound，得到 %v", err)
	}
}

func TestReorderSites(t *testing.T) {
	st := newStore(t)
	ids := make([]uint, 0, 3)
	for _, name := range []string{"A", "B", "C"} {
		site := &model.Site{Name: name, URL: "https://" + name + ".example.com"}
		if err := st.CreateSite(site); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, site.ID)
	}
	names := func() []string {
		sites, err := st.Sites()
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(sites))
		for _, s := range sites {
			out = append(out, s.Name)
		}
		return out
	}

	// 任意顺序：C、B、A
	if err := st.ReorderSites([]uint{ids[2], ids[1], ids[0]}); err != nil {
		t.Fatalf("整体重排失败：%v", err)
	}
	if got := names(); got[0] != "C" || got[1] != "B" || got[2] != "A" {
		t.Fatalf("重排未生效：%v", got)
	}
	sites, err := st.Sites()
	if err != nil {
		t.Fatal(err)
	}
	for i, site := range sites {
		if site.Sort != i+1 {
			t.Fatalf("sort 未重排为 1..N：%s 的 sort = %d", site.Name, site.Sort)
		}
	}

	if err := st.ReorderSites([]uint{ids[0], ids[1]}); err != ErrInvalidOrder {
		t.Fatalf("漏项应返回 ErrInvalidOrder，得到 %v", err)
	}
	if err := st.ReorderSites([]uint{ids[0], ids[0], ids[1]}); err != ErrInvalidOrder {
		t.Fatalf("重复项应返回 ErrInvalidOrder，得到 %v", err)
	}
	if err := st.ReorderSites([]uint{ids[0], ids[1], 9999}); err != ErrInvalidOrder {
		t.Fatalf("未知 id 应返回 ErrInvalidOrder，得到 %v", err)
	}
	if got := names(); got[0] != "C" || got[1] != "B" || got[2] != "A" {
		t.Fatalf("校验失败不应改动顺序：%v", got)
	}
}

func TestSettingsPasswordAndPurge(t *testing.T) {
	st := newStore(t)
	if err := st.EnsureDefaults(map[string]string{
		model.SettingAuthMode:  "private",
		model.SettingSiteTitle: "我的导航",
	}); err != nil {
		t.Fatal(err)
	}
	if got := st.Setting(model.SettingAuthMode, "public"); got != "private" {
		t.Fatalf("默认值应写入：%q", got)
	}
	// 已有值不应被覆盖
	if err := st.EnsureDefaults(map[string]string{model.SettingAuthMode: "public"}); err != nil {
		t.Fatal(err)
	}
	if got := st.Setting(model.SettingAuthMode, "public"); got != "private" {
		t.Fatalf("既有配置不应被默认值覆盖：%q", got)
	}

	if err := st.SetPasswordHash("hash-value"); err != nil {
		t.Fatal(err)
	}
	hash, err := st.PasswordHash()
	if err != nil || hash != "hash-value" {
		t.Fatalf("PasswordHash = %q, err=%v", hash, err)
	}

	if _, err := st.Seed(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WriteIconFile("deadbeef", ".png", []byte("data")); err != nil {
		t.Fatal(err)
	}
	path, ok := st.FindIconFile("deadbeef")
	if !ok {
		t.Fatal("应能找到图标文件")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("图标文件不存在：%v", err)
	}

	if err := st.PurgeAll(); err != nil {
		t.Fatal(err)
	}
	sites, _ := st.Sites()
	cats, _ := st.Categories()
	if len(sites) != 0 || len(cats) != 0 {
		t.Fatalf("清空后应为空：sites=%d cats=%d", len(sites), len(cats))
	}
	if _, ok := st.FindIconFile("deadbeef"); ok {
		t.Fatal("清空后图标文件应被移除")
	}
	if got := st.Setting(model.SettingSiteTitle, ""); got != "我的导航" {
		t.Fatalf("清空数据不应影响配置：%q", got)
	}

	// 清空后重启（重新打开同一数据库）不应重新播种
	if seeded, err := st.Seed(); err != nil || seeded {
		t.Fatalf("清空数据后不应重新播种：seeded=%v err=%v", seeded, err)
	}
}

// TestUpgradeFaviconSources 一次性升级只动「仍等于旧默认值」的库，用户改过的库不被覆盖。
func TestUpgradeFaviconSources(t *testing.T) {
	st := newStore(t)
	legacy := "hdicons,site,duckduckgo,google"
	next := "hdicons,nasicon,site,duckduckgo,google"

	// 库里没有这个设置：不升级（首次安装走 EnsureDefaults 写新默认值）
	if upgraded, err := st.UpgradeFaviconSources(legacy, next); err != nil || upgraded {
		t.Fatalf("无设置时不应升级：upgraded=%v err=%v", upgraded, err)
	}

	// 正好是旧默认值：升级成新默认值
	if err := st.SetSetting(model.SettingFaviconSources, legacy); err != nil {
		t.Fatal(err)
	}
	upgraded, err := st.UpgradeFaviconSources(legacy, next)
	if err != nil || !upgraded {
		t.Fatalf("旧默认值应升级：upgraded=%v err=%v", upgraded, err)
	}
	if got := st.Setting(model.SettingFaviconSources, ""); got != next {
		t.Errorf("升级后应为 %q，得到 %q", next, got)
	}
	// 再跑一次是幂等的
	if upgraded, err := st.UpgradeFaviconSources(legacy, next); err != nil || upgraded {
		t.Errorf("重复升级不应再改动：upgraded=%v err=%v", upgraded, err)
	}

	// 用户自己改过的值：不动
	custom := "site,duckduckgo"
	if err := st.SetSetting(model.SettingFaviconSources, custom); err != nil {
		t.Fatal(err)
	}
	if upgraded, err := st.UpgradeFaviconSources(legacy, next); err != nil || upgraded {
		t.Errorf("自定义值不应被覆盖：upgraded=%v err=%v", upgraded, err)
	}
	if got := st.Setting(model.SettingFaviconSources, ""); got != custom {
		t.Errorf("自定义值应保持不变，得到 %q", got)
	}
}

// TestCategoryIconColumnAndSeed 老库升级后 categories 自动补 icon 列；演示分类的图标留空（走自动匹配）。
func TestCategoryIconColumnAndSeed(t *testing.T) {
	st := newStore(t)

	var columns []struct {
		Name string
	}
	if err := st.DB().Raw("PRAGMA table_info(categories)").Scan(&columns).Error; err != nil {
		t.Fatalf("读取表结构失败：%v", err)
	}
	hasIcon := false
	for _, column := range columns {
		if column.Name == "icon" {
			hasIcon = true
		}
	}
	if !hasIcon {
		t.Fatal("categories 表应有 icon 列（AutoMigrate 自动补列，老库无需手工迁移）")
	}

	if _, err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	categories, err := st.Categories()
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) == 0 {
		t.Fatal("应有演示分类")
	}
	for _, category := range categories {
		if category.Icon != "" {
			t.Fatalf("演示分类「%s」的图标应留空（前端按分类名自动匹配），得到 %q", category.Name, category.Icon)
		}
	}

	// 手选图标要能落库并可清空（空串 = 回到自动匹配）
	updated, err := st.UpdateCategory(categories[0].ID, map[string]any{"icon": "shield"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Icon != "shield" {
		t.Fatalf("更新后 icon 应为 shield，得到 %q", updated.Icon)
	}
	cleared, err := st.UpdateCategory(categories[0].ID, map[string]any{"icon": ""})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Icon != "" {
		t.Fatalf("空串应清空图标，得到 %q", cleared.Icon)
	}
}

// TestDeletePrivateCategoryKeepsSitesPrivate 删除 / 清空私密分类后，其下「继承」的站点不能因为变成未分类而对访客公开。
func TestDeletePrivateCategoryKeepsSitesPrivate(t *testing.T) {
	st := newStore(t)
	secret := &model.Category{Name: "私密", Visibility: model.VisibilityPrivate}
	open := &model.Category{Name: "公开", Visibility: model.VisibilityPublic}
	for _, c := range []*model.Category{secret, open} {
		if err := st.CreateCategory(c); err != nil {
			t.Fatal(err)
		}
	}
	mk := func(name string, cat uint, vis string) uint {
		site := &model.Site{Name: name, CategoryID: &cat, Visibility: vis}
		if err := st.CreateSite(site); err != nil {
			t.Fatal(err)
		}
		return site.ID
	}
	inherit := mk("继承", secret.ID, model.VisibilityInherit)
	explicit := mk("显式私密", secret.ID, model.VisibilityPrivate)
	publicSite := mk("公开分类里的继承", open.ID, model.VisibilityInherit)

	if err := st.DeleteCategory(secret.ID); err != nil {
		t.Fatal(err)
	}
	check := func(id uint, wantVis string) {
		t.Helper()
		site, err := st.Site(id)
		if err != nil {
			t.Fatal(err)
		}
		if site.CategoryID != nil || site.Visibility != wantVis {
			t.Fatalf("站点 %s：category=%v visibility=%s，期望未分类且 %s", site.Name, site.CategoryID, site.Visibility, wantVis)
		}
	}
	check(inherit, model.VisibilityPrivate)
	check(explicit, model.VisibilityPrivate)

	// 清空全部分类：公开分类里的继承站点保持继承（仍然公开，符合预期），且不留悬空分类 ID
	secret2 := &model.Category{Name: "私密2", Visibility: model.VisibilityPrivate}
	if err := st.CreateCategory(secret2); err != nil {
		t.Fatal(err)
	}
	inherit2 := mk("继承2", secret2.ID, model.VisibilityInherit)
	if _, err := st.PurgeCategories(); err != nil {
		t.Fatal(err)
	}
	check(inherit2, model.VisibilityPrivate)
	check(publicSite, model.VisibilityInherit)
}
