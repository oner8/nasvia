package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/store"
	"github.com/oner8/nasvia/internal/visibility"
)

const maxIconStatusIDs = 100

type iconStatusDTO struct {
	ID         uint   `json:"id"`
	Icon       string `json:"icon"`
	IconState  string `json:"icon_state"`
	IconSource string `json:"icon_source"`
}

func parseIconStatusIDs(c *gin.Context) ([]uint, bool) {
	parts := strings.Split(c.Query("ids"), ",")
	if len(parts) == 0 || len(parts) > maxIconStatusIDs {
		fail(c, http.StatusBadRequest, "invalid_ids", "站点 ID 数量必须为 1–100 个")
		return nil, false
	}
	ids := make([]uint, 0, len(parts))
	seen := make(map[uint]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err != nil || value == 0 {
			fail(c, http.StatusBadRequest, "invalid_ids", "站点 ID 格式不正确")
			return nil, false
		}
		id := uint(value)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, true
}

func iconStatusVisible(row store.IconStatus, authenticated bool) bool {
	site := model.Site{ID: row.ID, CategoryID: row.CategoryID, Visibility: row.Visibility}
	var category *model.Category
	if row.CategoryID != nil {
		category = &model.Category{ID: *row.CategoryID, Visibility: row.CategoryVisibility}
	}
	return visibility.SiteVisible(site, category, authenticated)
}

func (s *Server) listIconStatuses(c *gin.Context, authenticated bool) {
	ids, valid := parseIconStatusIDs(c)
	if !valid {
		return
	}
	rows, err := s.store.IconStatuses(ids)
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "读取图标状态失败")
		return
	}
	items := make([]iconStatusDTO, 0, len(rows))
	for _, row := range rows {
		if !iconStatusVisible(row, authenticated) {
			continue
		}
		items = append(items, iconStatusDTO{
			ID:         row.ID,
			Icon:       siteIconPath(row.ID, row.IconHash, row.IconState),
			IconState:  row.IconState,
			IconSource: iconKind(row.IconSource, row.IconState),
		})
	}
	ok(c, gin.H{"items": items})
}

// handleListIconStatuses 仅返回当前身份可见站点的轻量图标状态。
func (s *Server) handleListIconStatuses(c *gin.Context) {
	if !s.readGate(c) {
		return
	}
	s.listIconStatuses(c, s.authenticated(c))
}

func (s *Server) handleAdminListIconStatuses(c *gin.Context) {
	s.listIconStatuses(c, true)
}

// handleSiteIcon 输出站点图标。可见性不通过时返回 404（不泄露站点是否存在），
// 无缓存时先返回占位图标并异步抓取。
func (s *Server) handleSiteIcon(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	site, err := s.store.Site(id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	authenticated := s.authenticated(c)
	cats, err := s.store.Categories()
	// 读不到分类时按不可见处理（fail-closed），否则私密分类下的站点图标会漏给访客
	if err != nil || !visibility.SiteVisible(*site, categoryOf(*site, categoryIndex(cats)), authenticated) {
		c.Status(http.StatusNotFound)
		return
	}

	immutable := site.IconHash != "" && c.Query("v") == iconVersion(site.IconHash)
	if site.IconHash != "" && s.serveCachedIcon(c, site.IconHash, immutable) {
		return
	}
	if site.IconState == "" || site.IconState == model.IconStatePending {
		s.triggerIconFetch(*site)
	}
	s.servePlaceholder(c, site.Name)
}
