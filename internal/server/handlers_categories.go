package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/store"
	"github.com/oner8/nasvia/internal/visibility"
)

// handleListCategories 前台分类列表（含当前身份可见的站点计数）。
func (s *Server) handleListCategories(c *gin.Context) {
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
	_, views := visibility.Apply(sites, cats, authenticated)
	ok(c, gin.H{"items": views, "total": len(views), "authenticated": authenticated})
}

// handleAdminListCategories 后台分类列表（计数包含全部站点）。
func (s *Server) handleAdminListCategories(c *gin.Context) {
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
	_, views := visibility.Apply(sites, cats, true)
	ok(c, gin.H{"items": views, "total": len(views), "authenticated": true})
}

type categoryInput struct {
	Name       *string `json:"name"`
	Visibility *string `json:"visibility"`
	// Icon 分类图标键（空串 = 清除，改为按分类名自动匹配）。
	Icon *string `json:"icon"`
}

func (s *Server) handleCreateCategory(c *gin.Context) {
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	name := ""
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if name == "" {
		fail(c, http.StatusBadRequest, "invalid_name", "分类名称不能为空")
		return
	}
	if len([]rune(name)) > 32 {
		fail(c, http.StatusBadRequest, "invalid_name", "分类名称过长")
		return
	}
	category := model.Category{
		Name:       name,
		Visibility: model.VisibilityPublic,
	}
	if in.Visibility != nil {
		category.Visibility = model.NormalizeCategoryVisibility(*in.Visibility)
	}
	if in.Icon != nil {
		icon := strings.TrimSpace(*in.Icon)
		if !model.ValidIconKey(icon) {
			fail(c, http.StatusBadRequest, "invalid_icon", "图标键只能是 1–32 位小写字母、数字或连字符")
			return
		}
		category.Icon = icon
	}
	if err := s.store.CreateCategory(&category); err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "保存分类失败")
		return
	}
	ok(c, gin.H{"category": visibility.CategoryView{Category: category}})
}

func (s *Server) handleUpdateCategory(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	fields := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			fail(c, http.StatusBadRequest, "invalid_name", "分类名称不能为空")
			return
		}
		fields["name"] = name
	}
	if in.Visibility != nil {
		fields["visibility"] = model.NormalizeCategoryVisibility(*in.Visibility)
	}
	if in.Icon != nil {
		icon := strings.TrimSpace(*in.Icon)
		if !model.ValidIconKey(icon) {
			fail(c, http.StatusBadRequest, "invalid_icon", "图标键只能是 1–32 位小写字母、数字或连字符")
			return
		}
		fields["icon"] = icon
	}
	updated, err := s.store.UpdateCategory(id, fields)
	if err != nil {
		if err == store.ErrNotFound {
			fail(c, http.StatusNotFound, "not_found", "分类不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "更新分类失败")
		return
	}
	sites, _ := s.store.Sites()
	ok(c, gin.H{"category": visibility.CategoryView{
		Category:  *updated,
		SiteCount: visibility.SiteCount(sites, *updated, true),
	}})
}

func (s *Server) handleDeleteCategory(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	if err := s.store.DeleteCategory(id); err != nil {
		if err == store.ErrNotFound {
			fail(c, http.StatusNotFound, "not_found", "分类不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "删除分类失败")
		return
	}
	ok(c, gin.H{"deleted": id})
}

func (s *Server) handleMoveCategory(c *gin.Context) {
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
	if err := s.store.MoveCategory(id, dir); err != nil {
		if err == store.ErrNotFound {
			fail(c, http.StatusNotFound, "not_found", "分类不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "store_error", "调整排序失败")
		return
	}
	sites, _ := s.store.Sites()
	cats, _ := s.store.Categories()
	_, views := visibility.Apply(sites, cats, true)
	ok(c, gin.H{"items": views})
}

func (s *Server) handlePurgeCategories(c *gin.Context) {
	count, err := s.store.PurgeCategories()
	if err != nil {
		fail(c, http.StatusInternalServerError, "store_error", "清空分类失败")
		return
	}
	ok(c, gin.H{"deleted": count})
}
