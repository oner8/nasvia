package server

import (
	"testing"

	"github.com/oner8/nasvia/internal/model"
)

// iconKind 归一化：前端据此决定要不要给图标再套一层灰底。
func TestIconKind(t *testing.T) {
	cases := []struct {
		source string
		state  string
		want   string
		why    string
	}{
		{"hdicons:jellyfin-1", model.IconStateReady, model.IconSourceHDIcons, "HD-Icons 图标自带圆角底"},
		{"HDIcons:Immich-1", model.IconStateReady, model.IconSourceHDIcons, "大小写不敏感"},
		{"placeholder", model.IconStateFailed, model.IconSourcePlaceholder, "内置占位图自带底"},
		{"", model.IconStateFailed, model.IconSourcePlaceholder, "抓取失败一律按占位图处理"},
		{"site", model.IconStateReady, model.IconSourceFavicon, "站点自身 favicon 需要底色"},
		{"duckduckgo", model.IconStateReady, model.IconSourceFavicon, "第三方 favicon 需要底色"},
		{"google", model.IconStateReady, model.IconSourceFavicon, "第三方 favicon 需要底色"},
		{"", model.IconStateReady, model.IconSourceFavicon, "历史数据（无来源）保守按 favicon"},
		{"", model.IconStatePending, model.IconSourceFavicon, "待抓取期间按 favicon"},
	}
	for _, tc := range cases {
		if got := iconKind(tc.source, tc.state); got != tc.want {
			t.Errorf("iconKind(%q, %q) = %q，期望 %q（%s）", tc.source, tc.state, got, tc.want, tc.why)
		}
	}
}
