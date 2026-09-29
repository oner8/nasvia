package server

import (
	"github.com/gin-gonic/gin"
)

// handleSuggest 联网搜索框的联想词：GET /api/suggest?engine=google&q=关键词。
// private 模式下同样要求登录（首页都看不到，也就用不到搜索框）。
func (s *Server) handleSuggest(c *gin.Context) {
	if !s.readGate(c) {
		return
	}
	items, source := s.suggest.Suggest(c.Request.Context(), c.Query("engine"), c.Query("q"))
	c.Header("Cache-Control", "private, max-age=300")
	ok(c, gin.H{"items": items, "source": source})
}
