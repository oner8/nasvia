package visibility

import (
	"testing"

	"github.com/oner8/nasvia/internal/model"
)

func cat(id uint, visibility string) model.Category {
	return model.Category{ID: id, Name: "c", Visibility: visibility}
}

func site(id uint, categoryID *uint, visibility string) model.Site {
	return model.Site{ID: id, Name: "s", CategoryID: categoryID, Visibility: visibility}
}

func ptr(v uint) *uint { return &v }

func TestSiteVisible(t *testing.T) {
	publicCat := cat(1, model.VisibilityPublic)
	privateCat := cat(2, model.VisibilityPrivate)

	cases := []struct {
		name          string
		site          model.Site
		category      *model.Category
		authenticated bool
		want          bool
	}{
		{"已登录可见全部", site(1, ptr(2), model.VisibilityPrivate), &privateCat, true, true},
		{"分类私密时显式公开也不可见", site(1, ptr(2), model.VisibilityPublic), &privateCat, false, false},
		{"分类公开时显式公开可见", site(1, ptr(1), model.VisibilityPublic), &publicCat, false, true},
		{"无分类的显式公开可见", site(1, nil, model.VisibilityPublic), nil, false, true},
		{"显式私密永远隐藏", site(1, ptr(1), model.VisibilityPrivate), &publicCat, false, false},
		{"继承公开分类", site(1, ptr(1), model.VisibilityInherit), &publicCat, false, true},
		{"继承私密分类", site(1, ptr(2), model.VisibilityInherit), &privateCat, false, false},
		{"无分类继承视为公开", site(1, nil, model.VisibilityInherit), nil, false, true},
		{"未知可见性按继承处理", site(1, ptr(2), ""), &privateCat, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SiteVisible(tc.site, tc.category, tc.authenticated); got != tc.want {
				t.Fatalf("SiteVisible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyFiltersAndCounts(t *testing.T) {
	publicCat := cat(1, model.VisibilityPublic)
	privateCat := cat(2, model.VisibilityPrivate)
	sites := []model.Site{
		site(1, ptr(1), model.VisibilityPublic),
		site(2, ptr(1), model.VisibilityPrivate),
		site(3, ptr(2), model.VisibilityPrivate),
		site(4, ptr(2), model.VisibilityInherit),
	}

	visible, views := Apply(sites, []model.Category{publicCat, privateCat}, false)
	if len(visible) != 1 || visible[0].ID != 1 {
		t.Fatalf("匿名可见站点错误：%+v", visible)
	}
	if len(views) != 1 || views[0].ID != publicCat.ID {
		t.Fatalf("匿名可见分类错误：%+v", views)
	}
	if views[0].SiteCount != 1 {
		t.Fatalf("分类计数应只统计可见站点，得到 %d", views[0].SiteCount)
	}

	all, allViews := Apply(sites, []model.Category{publicCat, privateCat}, true)
	if len(all) != 4 || len(allViews) != 2 {
		t.Fatalf("登录后应看到全部：sites=%d cats=%d", len(all), len(allViews))
	}
}

func TestPrivateCategoryCapsSitesAndIsNotPromoted(t *testing.T) {
	privateCat := cat(2, model.VisibilityPrivate)
	// 私密分类里即使存在显式公开的站点（历史数据），分类也不会对外可见。
	sites := []model.Site{
		site(1, ptr(2), model.VisibilityPublic),
		site(2, ptr(2), model.VisibilityPrivate),
		site(3, ptr(2), model.VisibilityInherit),
	}
	if CategoryVisible(privateCat, false) {
		t.Fatal("私密分类不应因站点的显式公开而对外可见")
	}
	if !CategoryVisible(privateCat, true) {
		t.Fatal("登录后分类应可见")
	}

	visible, views := Apply(sites, []model.Category{privateCat}, false)
	if len(visible) != 0 {
		t.Fatalf("私密分类下不应有匿名可见站点：%+v", visible)
	}
	if len(views) != 0 {
		t.Fatalf("私密分类不应出现在匿名分类列表里：%+v", views)
	}

	// 分类改为公开后，public / inherit 站点都恢复可见。
	publicCat := cat(2, model.VisibilityPublic)
	visible, views = Apply(sites, []model.Category{publicCat}, false)
	if len(visible) != 2 || len(views) != 1 || views[0].SiteCount != 2 {
		t.Fatalf("分类公开后应看到 public + inherit 两个站点：sites=%d views=%+v", len(visible), views)
	}
}

func TestSiteCount(t *testing.T) {
	publicCat := cat(1, model.VisibilityPublic)
	privateCat := cat(2, model.VisibilityPrivate)
	sites := []model.Site{
		site(1, ptr(1), model.VisibilityPublic),
		site(2, ptr(1), model.VisibilityPrivate),
		site(3, ptr(2), model.VisibilityInherit),
		site(4, ptr(2), model.VisibilityPublic),
	}
	if got := SiteCount(sites, publicCat, false); got != 1 {
		t.Fatalf("公开分类的匿名计数 = %d, want 1", got)
	}
	if got := SiteCount(sites, publicCat, true); got != 2 {
		t.Fatalf("公开分类的登录计数 = %d, want 2", got)
	}
	if got := SiteCount(sites, privateCat, false); got != 0 {
		t.Fatalf("私密分类的匿名计数应为 0（含显式公开站点），得到 %d", got)
	}
	if got := SiteCount(sites, privateCat, true); got != 2 {
		t.Fatalf("私密分类的登录计数 = %d, want 2", got)
	}
	if got := SiteCount(sites, cat(99, model.VisibilityPublic), true); got != 0 {
		t.Fatalf("空分类计数 = %d, want 0", got)
	}
}
