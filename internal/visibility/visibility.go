// Package visibility 实现服务端强制的可见性规则。
//
// 规则：
//   - 已登录（管理员会话）→ 可见全部内容。
//   - **分类是可见性的上限**：分类为 private 时，其下站点对访客一律不可见
//     （即使站点显式 public 也不可见，且服务端不允许这么设置，见 server 侧校验）。
//   - 分类 public 时：站点 public → 可见；private → 不可见；inherit → 跟随分类。
//   - 站点 inherit 且无分类 → 视为公开（避免新建站点莫名消失）。
//   - 分类自身 public → 对访客可见；private → 不可见（不因站点的显式 public 而提升）。
package visibility

import "github.com/oner8/nasvia/internal/model"

// CategoryView 分类 + 当前身份可见的站点计数。
type CategoryView struct {
	model.Category
	SiteCount int `json:"site_count"`
}

// SiteVisible 判断单个站点对当前请求者是否可见。
func SiteVisible(site model.Site, category *model.Category, authenticated bool) bool {
	if authenticated {
		return true
	}
	if site.Visibility == model.VisibilityPrivate {
		return false
	}
	// 显式 public 与 inherit 一样受所属分类上限约束：分类私密 → 不可见；无分类 → 可见。
	if category == nil {
		return true
	}
	return category.Visibility == model.VisibilityPublic
}

// CategoryVisible 判断分类对当前请求者是否可见；站点级别的 public 不会提升私密分类。
func CategoryVisible(category model.Category, authenticated bool) bool {
	if authenticated {
		return true
	}
	return category.Visibility == model.VisibilityPublic
}

// Apply 过滤站点与分类，并计算可见站点计数。
func Apply(sites []model.Site, categories []model.Category, authenticated bool) ([]model.Site, []CategoryView) {
	byID := make(map[uint]*model.Category, len(categories))
	for i := range categories {
		byID[categories[i].ID] = &categories[i]
	}

	visible := make([]model.Site, 0, len(sites))
	counts := make(map[uint]int, len(categories))
	for _, s := range sites {
		var cat *model.Category
		if s.CategoryID != nil {
			cat = byID[*s.CategoryID]
		}
		if !SiteVisible(s, cat, authenticated) {
			continue
		}
		visible = append(visible, s)
		if cat != nil {
			counts[cat.ID]++
		}
	}

	views := make([]CategoryView, 0, len(categories))
	for _, c := range categories {
		if !CategoryVisible(c, authenticated) {
			continue
		}
		views = append(views, CategoryView{Category: c, SiteCount: counts[c.ID]})
	}
	return visible, views
}

// SiteCount 返回分类中当前身份可见的站点数；分类私密时对访客恒为 0。
func SiteCount(sites []model.Site, category model.Category, authenticated bool) int {
	n := 0
	for _, s := range sites {
		if s.CategoryID == nil || *s.CategoryID != category.ID {
			continue
		}
		if SiteVisible(s, &category, authenticated) {
			n++
		}
	}
	return n
}
