package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/visibility"
)

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
