// Package model 定义 NASVIA 的数据模型与共享常量。
package model

import "time"

// 可见性取值：站点三态（public/private/inherit），分类两态（public/private）。
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
	VisibilityInherit = "inherit"
)

// 前台访问模式。
const (
	AuthModePublic  = "public"
	AuthModePrivate = "private"
)

// 图标抓取状态。
const (
	IconStatePending = "pending"
	IconStateReady   = "ready"
	IconStateFailed  = "failed"
	// 图标来源（icon_source）：记录这份图标从哪来，前端据此决定要不要再套一层底色。
	// 存的是原始来源（「hdicons:<条目名>」/「nasicon:<文件名>」），对外由 DTO 归一成几档。
	IconSourcePlaceholder = "placeholder" // 内置占位图（自带底）
	IconSourceHDIcons     = "hdicons"     // HD-Icons 圆角图标（自带圆角底）
	IconSourceNASIcon     = "nasicon"     // nasicon.top 图标（多为自带底的整图，不套灰底）
	IconSourceFavicon     = "favicon"     // 站点/第三方 favicon（多半透明，需要底色衬托）
)

// settings 表键名。
const (
	SettingInstalled = "installed"
	SettingAuthMode  = "auth_mode"
	SettingLANCIDRs  = "lan_cidrs"
	// SettingHomeEgress 家庭公网出口（IP / CIDR / DDNS 域名 / auto），服务端按访客 IP 判定内外网用。
	SettingHomeEgress     = "home_egress"
	SettingSiteTitle      = "site_title"
	SettingFaviconSources = "favicon_sources"
	SettingHDIconsMirrors = "hdicons_mirrors"
	SettingNASIconBase    = "nasicon_base"
	SettingPasswordHash   = "password_hash"
	SettingNavPosition    = "nav_position"
	SettingNavStyle       = "nav_style"
	SettingNavVisible     = "nav_visible"
)

// Category 服务分类。
type Category struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"size:64;not null" json:"name"`
	Sort       int    `gorm:"index;not null;default:0" json:"sort"`
	Visibility string `gorm:"size:16;not null;default:public" json:"visibility"`
	// Icon 分类在导航里的图标键（空 = 按分类名自动匹配，见 web/src/lib/category-icon.ts）。
	Icon      string    `gorm:"size:32;not null;default:''" json:"icon"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Site 导航站点。
type Site struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Description string `gorm:"size:512" json:"description"`
	URL         string `gorm:"size:512" json:"url"`
	LanURL      string `gorm:"size:512" json:"lan_url"`
	IconHash    string `gorm:"size:64" json:"-"`
	IconType    string `gorm:"size:64" json:"-"`
	IconState   string `gorm:"size:16;not null;default:pending" json:"icon_state"`
	IconSource  string `gorm:"size:64" json:"-"`
	// IconName 手动指定的 HD-Icons 条目名（空 = 自动匹配）；后台可搜索索引后指定。
	IconName   string    `gorm:"size:64" json:"-"`
	CategoryID *uint     `gorm:"index" json:"category_id"`
	Sort       int       `gorm:"index;not null;default:0" json:"sort"`
	Pinned     bool      `gorm:"not null;default:false" json:"pinned"`
	Visibility string    `gorm:"size:16;not null;default:inherit" json:"visibility"`
	Tags       string    `gorm:"size:256" json:"tags"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Setting 键值配置。
type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"size:4096" json:"value"`
}

// IconCache 域名 -> 已缓存图标 的映射，避免匿名请求触发任意域名的抓取。
type IconCache struct {
	Domain      string    `gorm:"primaryKey;size:255" json:"domain"`
	Hash        string    `gorm:"size:64" json:"hash"`
	Ext         string    `gorm:"size:8" json:"ext"`
	ContentType string    `gorm:"size:128" json:"content_type"`
	Source      string    `gorm:"size:32" json:"source"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// NormalizeVisibility 归一化站点可见性，非法值回落到 inherit。
func NormalizeVisibility(v string) string {
	switch v {
	case VisibilityPublic, VisibilityPrivate, VisibilityInherit:
		return v
	default:
		return VisibilityInherit
	}
}

// NormalizeCategoryVisibility 归一化分类可见性，非法值回落到 public。
func NormalizeCategoryVisibility(v string) string {
	switch v {
	case VisibilityPublic, VisibilityPrivate:
		return v
	default:
		return VisibilityPublic
	}
}

// NormalizeAuthMode 归一化前台模式，非法值回落到 public。
func NormalizeAuthMode(v string) string {
	if v == AuthModePrivate {
		return AuthModePrivate
	}
	return AuthModePublic
}

// 分类导航的位置与标签样式（后台统一设置，见 SettingNavPosition / SettingNavStyle）。
const (
	NavPositionLeft  = "left"
	NavPositionRight = "right"

	NavStyleText = "text" // 显示分类名称
	NavStyleIcon = "icon" // 只显示图标

	// 分类导航是否显示（后台统一设置，所有访客一致）。
	NavVisibleShow = "show"
	NavVisibleHide = "hide"
)

// NormalizeNavPosition 归一化导航位置：只有 right 保留，其余（含历史值 top、空值、非法值）一律回落到 left。
func NormalizeNavPosition(v string) string {
	if v == NavPositionRight {
		return NavPositionRight
	}
	return NavPositionLeft
}

// NormalizeNavStyle 归一化导航标签样式：默认（未设置 / 空值 / 非法值）为 icon（仅图标），只有显式 text 才用文字。
func NormalizeNavStyle(v string) string {
	if v == NavStyleText {
		return NavStyleText
	}
	return NavStyleIcon
}

// ValidNavPosition 判断取值是否是合法的导航位置（供接口校验用）：只认 left / right。
func ValidNavPosition(v string) bool {
	return v == NavPositionLeft || v == NavPositionRight
}

// NormalizeNavVisible 归一化「是否显示分类导航」，非法值回落到 show。
func NormalizeNavVisible(v string) string {
	if v == NavVisibleHide {
		return NavVisibleHide
	}
	return NavVisibleShow
}

// ValidNavVisible 判断取值是否是合法的显示开关（供接口校验用）。
func ValidNavVisible(v string) bool {
	return v == NavVisibleShow || v == NavVisibleHide
}

// ValidNavStyle 判断取值是否是合法的标签样式（供接口校验用）。
func ValidNavStyle(v string) bool {
	return v == NavStyleText || v == NavStyleIcon
}

// ValidIconKey 校验分类图标键：小写字母 / 数字 / 连字符，最多 32 字符；空串表示「自动」。
func ValidIconKey(v string) bool {
	if v == "" {
		return true
	}
	if len(v) > 32 {
		return false
	}
	for _, r := range v {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}
