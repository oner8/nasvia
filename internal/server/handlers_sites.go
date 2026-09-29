package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/hdicons"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/nasicon"
	"github.com/oner8/nasvia/internal/store"
	"github.com/oner8/nasvia/internal/visibility"
)

// siteDTO 对外输出的站点结构；icon 始终为非空的可访问地址。
type siteDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	LanURL      string `json:"lan_url"`
	Icon        string `json:"icon"`
	IconState   string `json:"icon_state"`
	IconSource  string `json:"icon_source"`
	// IconName / IconAuto 只在后台接口返回（omitempty），前台与匿名响应不出现。
	IconName            string `json:"icon_name,omitempty"`
	IconAuto            string `json:"icon_auto,omitempty"`
	CategoryID          *uint  `json:"category_id"`
	Sort                int    `json:"sort"`
	Pinned              bool   `json:"pinned"`
	Visibility          string `json:"visibility"`
	EffectiveVisibility string `json:"effective_visibility"`
	Tags                string `json:"tags"`
}

// iconKind 把图标来源归一成前端用的几档：
//   - hdicons：HD-Icons 圆角图标，自带圆角底 → 前端不要再套底色；
//   - nasicon：nasicon.top 的整图（方形/自带底）→ 同样不套底色；
//   - placeholder：内置占位图，自带渐变底 → 同样不套；
//   - favicon：站点/第三方 favicon，多半是透明小图 → 前端套一层灰底方块衬托。
//
// 空来源（历史数据）按 favicon 处理：宁可多套一层底，也不要让透明小图裸奔。
func iconKind(source, state string) string {
	value := strings.ToLower(strings.TrimSpace(source))
	switch {
	case strings.HasPrefix(value, model.IconSourceHDIcons):
		return model.IconSourceHDIcons
	case strings.HasPrefix(value, model.IconSourceNASIcon):
		return model.IconSourceNASIcon
	case value == model.IconSourcePlaceholder, state == model.IconStateFailed:
		return model.IconSourcePlaceholder
	default:
		return model.IconSourceFavicon
	}
}

// iconVersion 图标地址里的内容版本：哈希前 8 位。
func iconVersion(iconHash string) string {
	if len(iconHash) > 8 {
		return iconHash[:8]
	}
	return iconHash
}

// siteIconPath 站点图标地址，带一个「内容版本」：
// 图标抓取完成后 hash 会变，URL 随之改变，浏览器就会立刻取新图，
// 而不是继续用创建时缓存下来的占位图（这是「新加的应用好像从不去找图标」的根因）。
func siteIconPath(id uint, iconHash, iconState string) string {
	version := iconVersion(iconHash)
	if version == "" {
		version = iconState
	}
	if version == "" {
		version = "pending"
	}
	return fmt.Sprintf("/api/sites/%d/icon?v=%s", id, url.QueryEscape(version))
}

func toSiteDTO(site model.Site, category *model.Category, authenticated bool) siteDTO {
	effective := site.Visibility
	if site.Visibility == model.VisibilityInherit {
		if category == nil {
			effective = model.VisibilityPublic
		} else {
			effective = category.Visibility
		}
	}
	return siteDTO{
		ID:                  site.ID,
		Name:                site.Name,
		Description:         site.Description,
		URL:                 site.URL,
		LanURL:              site.LanURL,
		Icon:                siteIconPath(site.ID, site.IconHash, site.IconState),
		IconState:           site.IconState,
		IconSource:          iconKind(site.IconSource, site.IconState),
		CategoryID:          site.CategoryID,
		Sort:                site.Sort,
		Pinned:              site.Pinned,
		Visibility:          site.Visibility,
		EffectiveVisibility: effective,
		Tags:                site.Tags,
	}
}

func toSiteDTOs(sites []model.Site, index map[uint]*model.Category, authenticated bool) []siteDTO {
	out := make([]siteDTO, 0, len(sites))
	for _, site := range sites {
		out = append(out, toSiteDTO(site, categoryOf(site, index), authenticated))
	}
	return out
}

func matchSite(site model.Site, query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(site.Name), q) ||
		strings.Contains(strings.ToLower(site.Description), q) ||
		strings.Contains(strings.ToLower(site.URL), q) ||
		strings.Contains(strings.ToLower(site.LanURL), q) ||
		strings.Contains(strings.ToLower(site.Tags), q)
}

// handleListSites 前台列表：public 模式匿名仅返回公开站点，private 模式未登录直接 401。
func (s *Server) handleListSites(c *gin.Context) {
	authenticated := s.authenticated(c)
	if !s.readGate(c) {
		return
	}
	sites, err := s.store.Sites()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "读取站点失败")
		return
	}
	cats, err := s.store.Categories()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "读取分类失败")
		return
	}
	visible, _ := visibility.Apply(sites, cats, authenticated)
	index := categoryIndex(cats)

	query := strings.TrimSpace(c.Query("q"))
	if query != "" {
		filtered := make([]model.Site, 0, len(visible))
		for _, site := range visible {
			if matchSite(site, query) {
				filtered = append(filtered, site)
			}
		}
		visible = filtered
	}
	ok(c, gin.H{
		"items":         toSiteDTOs(visible, index, authenticated),
		"total":         len(visible),
		"authenticated": authenticated,
	})
}

// handleAdminListSites 后台列表：登录后可见全部站点。
func (s *Server) handleAdminListSites(c *gin.Context) {
	sites, err := s.store.Sites()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "读取站点失败")
		return
	}
	cats, err := s.store.Categories()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "读取分类失败")
		return
	}
	ok(c, gin.H{
		"items":         s.adminSiteDTOs(sites, categoryIndex(cats)),
		"total":         len(sites),
		"authenticated": true,
	})
}

type siteInput struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	URL           *string `json:"url"`
	LanURL        *string `json:"lan_url"`
	CategoryID    *uint   `json:"category_id"`
	ClearCategory bool    `json:"clear_category"`
	Visibility    *string `json:"visibility"`
	Tags          *string `json:"tags"`
	Pinned        *bool   `json:"pinned"`
	// IconName 手动指定的 HD-Icons 条目名（空 = 自动匹配）；ClearIconName 显式清回自动匹配。
	IconName      *string `json:"icon_name"`
	ClearIconName bool    `json:"clear_icon_name"`
}

func normalizeAddress(raw string, field string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%s 不是合法地址", field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s 仅支持 http/https", field)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%s 缺少主机名", field)
	}
	return trimmed, nil
}

func (s *Server) handleCreateSite(c *gin.Context) {
	var in siteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	name := ""
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if name == "" {
		fail(c, http.StatusBadRequest, "invalid_name", "站点名称不能为空")
		return
	}
	if len([]rune(name)) > 64 {
		fail(c, http.StatusBadRequest, "invalid_name", "站点名称过长")
		return
	}
	external, lan := "", ""
	if in.URL != nil {
		parsed, err := normalizeAddress(*in.URL, "外网地址")
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid_url", err.Error())
			return
		}
		external = parsed
	}
	if in.LanURL != nil {
		parsed, err := normalizeAddress(*in.LanURL, "内网地址")
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid_url", err.Error())
			return
		}
		lan = parsed
	}
	if external == "" && lan == "" {
		fail(c, http.StatusBadRequest, "missing_url", "内网地址与外网地址至少填写一个")
		return
	}
	if in.CategoryID != nil {
		if _, err := s.store.Category(*in.CategoryID); err != nil {
			fail(c, http.StatusBadRequest, "invalid_category", "分类不存在")
			return
		}
	}

	site := model.Site{
		Name:        name,
		Description: valueOr(in.Description, ""),
		URL:         external,
		LanURL:      lan,
		CategoryID:  in.CategoryID,
		Visibility:  model.VisibilityInherit,
		Tags:        valueOr(in.Tags, ""),
		IconState:   model.IconStatePending,
	}
	if in.Visibility != nil {
		site.Visibility = model.NormalizeVisibility(*in.Visibility)
	}
	if in.IconName != nil {
		iconName, err := s.resolveIconName(*in.IconName)
		if err != nil {
			fail(c, http.StatusBadRequest, "unknown_icon", err.Error())
			return
		}
		site.IconName = iconName
	}
	if in.Pinned != nil {
		site.Pinned = *in.Pinned
	}
	if s.rejectPublicInPrivateCategory(c, site.CategoryID, site.Visibility) {
		return
	}
	if err := s.store.CreateSite(&site); err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "保存站点失败")
		return
	}
	if s.triggerIconFetch(site) == iconUnavailable {
		if refreshed, err := s.store.Site(site.ID); err == nil {
			site = *refreshed
		}
	}
	ok(c, gin.H{"site": s.withIconDebug(toSiteDTO(site, nil, true), site)})
}

func (s *Server) handleUpdateSite(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	existing, err := s.store.Site(id)
	if err != nil {
		fail(c, http.StatusNotFound, "not_found", "站点不存在")
		return
	}
	var in siteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	fields := map[string]any{}
	addressChanged := false
	iconChanged := false

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			fail(c, http.StatusBadRequest, "invalid_name", "站点名称不能为空")
			return
		}
		fields["name"] = name
	}
	if in.Description != nil {
		fields["description"] = strings.TrimSpace(*in.Description)
	}
	if in.Tags != nil {
		fields["tags"] = strings.TrimSpace(*in.Tags)
	}
	if in.ClearIconName {
		fields["icon_name"] = ""
		iconChanged = true
	} else if in.IconName != nil {
		name, err := s.resolveIconName(*in.IconName)
		if err != nil {
			fail(c, http.StatusBadRequest, "unknown_icon", err.Error())
			return
		}
		fields["icon_name"] = name
		iconChanged = true
	}
	if in.Pinned != nil {
		fields["pinned"] = *in.Pinned
	}
	if in.Visibility != nil {
		fields["visibility"] = model.NormalizeVisibility(*in.Visibility)
	}
	if in.URL != nil {
		parsed, err := normalizeAddress(*in.URL, "外网地址")
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid_url", err.Error())
			return
		}
		fields["url"] = parsed
		addressChanged = true
	}
	if in.LanURL != nil {
		parsed, err := normalizeAddress(*in.LanURL, "内网地址")
		if err != nil {
			fail(c, http.StatusBadRequest, "invalid_url", err.Error())
			return
		}
		fields["lan_url"] = parsed
		addressChanged = true
	}
	if in.ClearCategory {
		fields["category_id"] = nil
	} else if in.CategoryID != nil {
		if _, err := s.store.Category(*in.CategoryID); err != nil {
			fail(c, http.StatusBadRequest, "invalid_category", "分类不存在")
			return
		}
		fields["category_id"] = *in.CategoryID
	}

	nextCategoryID := existing.CategoryID
	if raw, has := fields["category_id"]; has {
		if raw == nil {
			nextCategoryID = nil
		} else if id, isUint := raw.(uint); isUint {
			nextCategoryID = &id
		}
	}
	nextVisibility := existing.Visibility
	if v, has := fields["visibility"].(string); has {
		nextVisibility = v
	}
	if s.rejectPublicInPrivateCategory(c, nextCategoryID, nextVisibility) {
		return
	}

	nextURL := existing.URL
	if v, has := fields["url"].(string); has {
		nextURL = v
	}
	nextLan := existing.LanURL
	if v, has := fields["lan_url"].(string); has {
		nextLan = v
	}
	if strings.TrimSpace(nextURL) == "" && strings.TrimSpace(nextLan) == "" {
		fail(c, http.StatusBadRequest, "missing_url", "内网地址与外网地址至少填写一个")
		return
	}
	if addressChanged || iconChanged {
		fields["icon_state"] = model.IconStatePending
		fields["icon_hash"] = ""
		fields["icon_type"] = ""
	}

	updated, err := s.store.UpdateSite(id, fields)
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "更新站点失败")
		return
	}
	if addressChanged || iconChanged {
		if s.triggerIconFetch(*updated) == iconUnavailable {
			if refreshed, err := s.store.Site(updated.ID); err == nil {
				updated = refreshed
			}
		}
	}
	cats, _ := s.store.Categories()
	ok(c, gin.H{"site": s.withIconDebug(toSiteDTO(*updated, categoryOf(*updated, categoryIndex(cats)), true), *updated)})
}

func (s *Server) handleDeleteSite(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	if err := s.store.DeleteSite(id); err != nil {
		if err == store.ErrNotFound {
			fail(c, http.StatusNotFound, "not_found", "站点不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "删除站点失败")
		return
	}
	ok(c, gin.H{"deleted": id})
}

type batchInput struct {
	IDs           []uint  `json:"ids"`
	Action        string  `json:"action"`
	Visibility    *string `json:"visibility"`
	Pinned        *bool   `json:"pinned"`
	CategoryID    *uint   `json:"category_id"`
	ClearCategory bool    `json:"clear_category"`
}

func (s *Server) handleBatchSites(c *gin.Context) {
	var in batchInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	if len(in.IDs) == 0 {
		fail(c, http.StatusBadRequest, "empty_selection", "请先选择站点")
		return
	}
	switch in.Action {
	case "delete":
		count, err := s.store.DeleteSites(in.IDs)
		if err != nil {
			fail(c, http.StatusInternalServerError, "store_error", "批量删除失败")
			return
		}
		ok(c, gin.H{"affected": count})
	case "visibility":
		if in.Visibility == nil {
			fail(c, http.StatusBadRequest, "invalid_body", "缺少 visibility")
			return
		}
		if model.NormalizeVisibility(*in.Visibility) == model.VisibilityPublic {
			// 私密分类下的站点不允许设为公开
			sites, err := s.store.Sites()
			if err != nil {
				fail(c, http.StatusInternalServerError, "store_error", "读取站点失败")
				return
			}
			byID := make(map[uint]model.Site, len(sites))
			for _, item := range sites {
				byID[item.ID] = item
			}
			for _, id := range in.IDs {
				item, exists := byID[id]
				if !exists {
					continue
				}
				if s.rejectPublicInPrivateCategory(c, item.CategoryID, model.VisibilityPublic) {
					return
				}
			}
		}
		affected, err := s.updateMany(in.IDs, map[string]any{"visibility": model.NormalizeVisibility(*in.Visibility)})
		if err != nil {
			fail(c, http.StatusInternalServerError, "store_error", "批量更新失败")
			return
		}
		ok(c, gin.H{"affected": affected})
	case "pinned":
		if in.Pinned == nil {
			fail(c, http.StatusBadRequest, "invalid_body", "缺少 pinned")
			return
		}
		affected, err := s.updateMany(in.IDs, map[string]any{"pinned": *in.Pinned})
		if err != nil {
			fail(c, http.StatusInternalServerError, "store_error", "批量更新失败")
			return
		}
		ok(c, gin.H{"affected": affected})
	case "category":
		value := any(nil)
		if !in.ClearCategory && in.CategoryID != nil {
			value = *in.CategoryID
		}
		if value != nil {
			target, err := s.store.Category(value.(uint))
			if err != nil {
				// 不存在的分类 ID 会让站点「看似有分类、实则按无分类算」，私密判定随之失效
				fail(c, http.StatusBadRequest, "invalid_category", "分类不存在")
				return
			}
			if target.Visibility == model.VisibilityPrivate {
				// 目标分类是私密分类：选中站点里不能有显式公开的
				sites, _ := s.store.Sites()
				byID := make(map[uint]model.Site, len(sites))
				for _, item := range sites {
					byID[item.ID] = item
				}
				for _, id := range in.IDs {
					item, exists := byID[id]
					if !exists {
						continue
					}
					if item.Visibility == model.VisibilityPublic {
						fail(c, http.StatusBadRequest, "public_in_private_category",
							"目标分类是私密分类，请先把选中的站点改为「私密」或「继承」")
						return
					}
				}
			}
		}
		affected, err := s.updateMany(in.IDs, map[string]any{"category_id": value})
		if err != nil {
			fail(c, http.StatusInternalServerError, "store_error", "批量更新失败")
			return
		}
		ok(c, gin.H{"affected": affected})
	default:
		fail(c, http.StatusBadRequest, "invalid_action", "不支持的批量操作")
	}
}

func (s *Server) updateMany(ids []uint, fields map[string]any) (int64, error) {
	var affected int64
	for _, id := range ids {
		res := s.store.DB().Model(&model.Site{}).Where("id = ?", id).Updates(fields)
		if res.Error != nil {
			return affected, res.Error
		}
		affected += res.RowsAffected
	}
	return affected, nil
}

// rejectPublicInPrivateCategory 校验「分类私密时站点不能设为公开」。
// 分类是可见性的上限：私密分类里的公开站点既不会对外可见，也不该被配出来。
func (s *Server) rejectPublicInPrivateCategory(c *gin.Context, categoryID *uint, visibility string) bool {
	if visibility != model.VisibilityPublic || categoryID == nil {
		return false
	}
	category, err := s.store.Category(*categoryID)
	if err != nil || category == nil || category.Visibility != model.VisibilityPrivate {
		return false
	}
	fail(c, http.StatusBadRequest, "public_in_private_category",
		"所属分类是私密分类，站点不能单独设为公开；请改为「私密」或「继承」")
	return true
}

func (s *Server) handlePurgeSites(c *gin.Context) {
	count, err := s.store.PurgeSites()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "清空站点失败")
		return
	}
	ok(c, gin.H{"deleted": count})
}

type moveInput struct {
	Direction string `json:"direction"`
}

func (s *Server) handleMoveSite(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	var in moveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	dir := 0
	switch in.Direction {
	case "up":
		dir = -1
	case "down":
		dir = 1
	default:
		fail(c, http.StatusBadRequest, "invalid_direction", "direction 只能是 up 或 down")
		return
	}
	if err := s.store.MoveSite(id, dir); err != nil {
		if err == store.ErrNotFound {
			fail(c, http.StatusNotFound, "not_found", "站点不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "调整排序失败")
		return
	}
	sites, _ := s.store.Sites()
	cats, _ := s.store.Categories()
	ok(c, gin.H{"items": s.adminSiteDTOs(sites, categoryIndex(cats))})
}

// reorderInput 拖动排序：ids 是全部站点的目标顺序。
type reorderInput struct {
	IDs []uint `json:"ids"`
}

// handleReorderSites 按前端拖动后的顺序整体重排站点；ids 必须不重不漏地覆盖全部站点。
func (s *Server) handleReorderSites(c *gin.Context) {
	var in reorderInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	if err := s.store.ReorderSites(in.IDs); err != nil {
		if err == store.ErrInvalidOrder {
			fail(c, http.StatusBadRequest, "invalid_order", "排序列表必须不重不漏地包含全部站点")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "调整排序失败")
		return
	}
	sites, _ := s.store.Sites()
	cats, _ := s.store.Categories()
	ok(c, gin.H{"items": s.adminSiteDTOs(sites, categoryIndex(cats))})
}

func (s *Server) handleRefetchIcon(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	site, err := s.store.Site(id)
	if err != nil {
		fail(c, http.StatusNotFound, "not_found", "站点不存在")
		return
	}
	result, err := s.queueIconFetch(*site, true)
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "重置图标状态失败")
		return
	}
	switch result {
	case iconAlreadyQueued:
		fail(c, http.StatusConflict, "icon_fetch_in_progress", "该站点的图标正在抓取")
	case iconUnavailable:
		fail(c, http.StatusBadRequest, "icon_fetch_unavailable", "没有可抓取的地址，或所有图标来源均已关闭")
	default:
		ok(c, gin.H{"queued": true})
	}
}

func valueOr(ptr *string, fallback string) string {
	if ptr == nil {
		return fallback
	}
	return strings.TrimSpace(*ptr)
}

// resolveIconName 校验后台手动指定的图标名：本地已有对应索引缓存时要求能解析到条目，
// 免得存进一个永远配不上的名字；缓存还没拉到时先原样接受，抓取时再判定。
// nasicon 条目用「nasicon:<文件名>」前缀显式指定，裸名仍按 HD-Icons 解析。
func (s *Server) resolveIconName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if filename, ok := strings.CutPrefix(value, nasiconPrefix); ok {
		index := s.nasIndexCached()
		if index == nil {
			return value, nil
		}
		if item := nasicon.Resolve(index.Items, filename); item.Filename != "" {
			return nasiconPrefix + item.Filename, nil
		}
		return "", fmt.Errorf("nasicon 索引里没有「%s」，请在图标搜索里挑一个（留空则自动匹配）", filename)
	}
	index := s.hdIndexCached()
	if index == nil {
		return value, nil
	}
	if name := hdicons.Resolve(index.Items, value); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("索引里没有「%s」，请在图标搜索里挑一个（留空则自动匹配）", value)
}

// withIconDebug 补上后台用的「手动指定 / 自动匹配」两个字段；前台接口不调用，不对外暴露。
func (s *Server) withIconDebug(dto siteDTO, site model.Site) siteDTO {
	dto.IconName = site.IconName
	dto.IconAuto = s.autoIconName(site)
	return dto
}

// adminSiteDTOs 后台站点列表：附图标诊断字段。
func (s *Server) adminSiteDTOs(sites []model.Site, index map[uint]*model.Category) []siteDTO {
	out := make([]siteDTO, 0, len(sites))
	for _, site := range sites {
		out = append(out, s.withIconDebug(toSiteDTO(site, categoryOf(site, index), true), site))
	}
	return out
}
