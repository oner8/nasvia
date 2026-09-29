// Package store 封装 SQLite 持久化（GORM + glebarez/sqlite 纯 Go 驱动）。
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/oner8/nasvia/internal/model"
)

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("record not found")

// ErrInvalidOrder 排序列表与库内数据不一致（漏项或重复）。
var ErrInvalidOrder = errors.New("invalid order")

// Store 数据访问层。
type Store struct {
	db       *gorm.DB
	iconsDir string
}

// Open 打开数据库并准备图标目录；默认使用回滚日志（非 WAL），保证运行时只有单个 .db 文件。
func Open(dbPath, iconsDir string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create icons dir: %w", err)
	}
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if _, err := sqlDB.Exec("PRAGMA journal_mode = DELETE"); err != nil {
		return nil, fmt.Errorf("set journal mode: %w", err)
	}
	return &Store{db: db, iconsDir: iconsDir}, nil
}

// DB 暴露底层句柄（测试使用）。
func (s *Store) DB() *gorm.DB { return s.db }

// IconsDir 图标缓存目录。
func (s *Store) IconsDir() string { return s.iconsDir }

// Close 关闭数据库连接。
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate 建立/升级表结构。
func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&model.Category{}, &model.Site{}, &model.Setting{}, &model.IconCache{})
}

// EnsureDefaults 仅在键缺失时写入默认值（数据库中的既有值优先）。
func (s *Store) EnsureDefaults(defaults map[string]string) error {
	for key, value := range defaults {
		if strings.TrimSpace(value) == "" {
			continue
		}
		exists, err := s.hasSetting(key)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := s.SetSetting(key, value); err != nil {
			return err
		}
	}
	return nil
}

// UpgradeFaviconSources 把仍等于 legacy 的来源设置换成 next（一次性升级：给老库补上 nasicon 兜底来源）。
// 只认「正好等于旧默认值」的库：用户自己改过来源开关的库不会被覆盖。
func (s *Store) UpgradeFaviconSources(legacy, next string) (bool, error) {
	legacy = strings.TrimSpace(legacy)
	next = strings.TrimSpace(next)
	if legacy == "" || next == "" || legacy == next {
		return false, nil
	}
	if current := strings.TrimSpace(s.Setting(model.SettingFaviconSources, "")); current != legacy {
		return false, nil
	}
	if err := s.SetSetting(model.SettingFaviconSources, next); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) hasSetting(key string) (bool, error) {
	var count int64
	if err := s.db.Model(&model.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Settings 返回全部配置。
func (s *Store) Settings() (map[string]string, error) {
	var rows []model.Setting
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

// Setting 读取单个配置，缺失时返回 fallback。
func (s *Store) Setting(key, fallback string) string {
	var row model.Setting
	if err := s.db.Where("key = ?", key).First(&row).Error; err != nil {
		return fallback
	}
	if strings.TrimSpace(row.Value) == "" {
		return fallback
	}
	return row.Value
}

// SetSetting 写入单个配置。
func (s *Store) SetSetting(key, value string) error {
	return s.db.Save(&model.Setting{Key: key, Value: value}).Error
}

// SetSettings 原子写入多个配置。
func (s *Store) SetSettings(kv map[string]string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range kv {
			if err := tx.Save(&model.Setting{Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// IsInstalled 是否已完成首次初始化（用于避免清空数据后重新播种）。
func (s *Store) IsInstalled() bool {
	ok, err := s.hasSetting(model.SettingInstalled)
	return err == nil && ok
}

// MarkInstalled 标记已完成初始化。
func (s *Store) MarkInstalled() error {
	return s.SetSetting(model.SettingInstalled, time.Now().UTC().Format(time.RFC3339))
}

// PasswordHash 返回数据库中保存的密码散列（未设置时为空）。
func (s *Store) PasswordHash() (string, error) {
	var row model.Setting
	err := s.db.Where("key = ?", model.SettingPasswordHash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Value, nil
}

// SetPasswordHash 保存密码散列。
func (s *Store) SetPasswordHash(hash string) error {
	return s.SetSetting(model.SettingPasswordHash, hash)
}

// ---------------------------------------------------------------- 分类

// Categories 按排序返回全部分类。
func (s *Store) Categories() ([]model.Category, error) {
	var rows []model.Category
	if err := s.db.Order("sort asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Category 读取单个分类。
func (s *Store) Category(id uint) (*model.Category, error) {
	var row model.Category
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// CreateCategory 新建分类，自动置于末尾。
func (s *Store) CreateCategory(c *model.Category) error {
	var maxSort *int
	if err := s.db.Model(&model.Category{}).Select("MAX(sort)").Scan(&maxSort).Error; err != nil {
		return err
	}
	if maxSort != nil {
		c.Sort = *maxSort + 1
	} else {
		c.Sort = 1
	}
	return s.db.Create(c).Error
}

// UpdateCategory 局部更新分类。
func (s *Store) UpdateCategory(id uint, fields map[string]any) (*model.Category, error) {
	if len(fields) > 0 {
		res := s.db.Model(&model.Category{}).Where("id = ?", id).Updates(fields)
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			if _, err := s.Category(id); err != nil {
				return nil, err
			}
		}
	}
	return s.Category(id)
}

// DeleteCategory 删除分类，其下站点变为“未分类”（不删除站点）。
func (s *Store) DeleteCategory(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := keepInheritedPrivate(tx, []uint{id}); err != nil {
			return err
		}
		if err := tx.Model(&model.Site{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
			return err
		}
		res := tx.Delete(&model.Category{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// MoveCategory 上移/下移分类（dir = -1 或 1）。
func (s *Store) MoveCategory(id uint, dir int) error {
	rows, err := s.Categories()
	if err != nil {
		return err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	moved, err := moveInSlice(ids, id, dir)
	if err != nil {
		return err
	}
	if !moved {
		return nil
	}
	return s.reorderCategories(ids)
}

func (s *Store) reorderCategories(ids []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&model.Category{}).Where("id = ?", id).Update("sort", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------- 站点

// Sites 按排序返回全部站点。
func (s *Store) Sites() ([]model.Site, error) {
	var rows []model.Site
	if err := s.db.Order("sort asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Site 读取单个站点。
func (s *Store) Site(id uint) (*model.Site, error) {
	var row model.Site
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// IconStatus 是图标状态轮询所需的最小字段集合。
type IconStatus struct {
	ID                 uint
	IconHash           string
	IconState          string
	IconSource         string
	CategoryID         *uint
	Visibility         string
	CategoryVisibility string
}

// IconStatuses 只读取指定站点的图标与可见性字段，避免轮询完整站点列表。
func (s *Store) IconStatuses(ids []uint) ([]IconStatus, error) {
	if len(ids) == 0 {
		return []IconStatus{}, nil
	}
	var rows []IconStatus
	err := s.db.Table("sites").
		Select("sites.id, sites.icon_hash, sites.icon_state, sites.icon_source, sites.category_id, sites.visibility, COALESCE(categories.visibility, '') AS category_visibility").
		Joins("LEFT JOIN categories ON categories.id = sites.category_id").
		Where("sites.id IN ?", ids).
		Order("sites.id ASC").
		Scan(&rows).Error
	return rows, err
}

// CreateSite 新建站点，自动置于末尾。
func (s *Store) CreateSite(site *model.Site) error {
	var maxSort *int
	if err := s.db.Model(&model.Site{}).Select("MAX(sort)").Scan(&maxSort).Error; err != nil {
		return err
	}
	if maxSort != nil {
		site.Sort = *maxSort + 1
	} else {
		site.Sort = 1
	}
	if site.IconState == "" {
		site.IconState = model.IconStatePending
	}
	return s.db.Create(site).Error
}

// UpdateSite 局部更新站点。
func (s *Store) UpdateSite(id uint, fields map[string]any) (*model.Site, error) {
	if len(fields) > 0 {
		res := s.db.Model(&model.Site{}).Where("id = ?", id).Updates(fields)
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			if _, err := s.Site(id); err != nil {
				return nil, err
			}
		}
	}
	return s.Site(id)
}

// DeleteSite 删除站点。
func (s *Store) DeleteSite(id uint) error {
	res := s.db.Delete(&model.Site{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSites 批量删除站点，返回删除数量。
func (s *Store) DeleteSites(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := s.db.Delete(&model.Site{}, ids)
	return res.RowsAffected, res.Error
}

// PurgeSites 清空全部站点。
func (s *Store) PurgeSites() (int64, error) {
	res := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Site{})
	return res.RowsAffected, res.Error
}

// PurgeCategories 清空全部分类。
func (s *Store) PurgeCategories() (int64, error) {
	var affected int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := keepInheritedPrivate(tx, nil); err != nil {
			return err
		}
		// 站点一律变为未分类，不留指向已删除分类的悬空 ID
		if err := tx.Model(&model.Site{}).Where("category_id IS NOT NULL").Update("category_id", nil).Error; err != nil {
			return err
		}
		res := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Category{})
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

// keepInheritedPrivate 删除分类前调用：categoryIDs 里（nil 表示全部分类）凡是私密的，其下「继承」可见性的站点
// 改为显式「私密」。否则分类一删、站点变成「继承 + 未分类」，就会被当作公开，对匿名访客突然可见。
func keepInheritedPrivate(tx *gorm.DB, categoryIDs []uint) error {
	query := tx.Model(&model.Category{}).Where("visibility = ?", model.VisibilityPrivate)
	if categoryIDs != nil {
		query = query.Where("id IN ?", categoryIDs)
	}
	var privateIDs []uint
	if err := query.Pluck("id", &privateIDs).Error; err != nil {
		return err
	}
	if len(privateIDs) == 0 {
		return nil
	}
	return tx.Model(&model.Site{}).
		Where("visibility = ? AND category_id IN ?", model.VisibilityInherit, privateIDs).
		Update("visibility", model.VisibilityPrivate).Error
}

// PurgeAll 清空站点、分类与图标缓存（含磁盘图标文件）。
func (s *Store) PurgeAll() error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Site{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Category{}).Error; err != nil {
			return err
		}
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.IconCache{}).Error
	})
	if err != nil {
		return err
	}
	return s.RemoveIconFiles()
}

// MoveSite 上移/下移站点（dir = -1 或 1）。
func (s *Store) MoveSite(id uint, dir int) error {
	rows, err := s.Sites()
	if err != nil {
		return err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	moved, err := moveInSlice(ids, id, dir)
	if err != nil {
		return err
	}
	if !moved {
		return nil
	}
	return s.reorderSites(ids)
}

func (s *Store) reorderSites(ids []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&model.Site{}).Where("id = ?", id).Update("sort", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReorderSites 按给定顺序重排全部站点：ids 必须与库内站点集合完全一致（不重不漏），
// 否则返回 ErrInvalidOrder——避免前端漏传导致部分站点 sort 重复。
func (s *Store) ReorderSites(ids []uint) error {
	rows, err := s.Sites()
	if err != nil {
		return err
	}
	if len(ids) != len(rows) {
		return ErrInvalidOrder
	}
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return ErrInvalidOrder
		}
		seen[id] = struct{}{}
	}
	for _, row := range rows {
		if _, exists := seen[row.ID]; !exists {
			return ErrInvalidOrder
		}
	}
	return s.reorderSites(ids)
}

// SetSiteIconFailed 记录抓取失败，但仍保存占位图哈希：这样图标接口始终有本地缓存可返回，
// 同时保留 failed 状态供后台显示「抓取失败，可重试」；占位图自带底，前端无需再套底色。
func (s *Store) SetSiteIconFailed(id uint, hash, contentType string) error {
	return s.db.Model(&model.Site{}).Where("id = ?", id).Updates(map[string]any{
		"icon_hash":   hash,
		"icon_type":   contentType,
		"icon_state":  model.IconStateFailed,
		"icon_source": model.IconSourcePlaceholder,
	}).Error
}

// SetSiteIcon 记录站点图标缓存信息；source 记录图标来源（favicon 来源 / hdicons:<条目名> 等）。
func (s *Store) SetSiteIcon(id uint, hash, contentType, source string) error {
	return s.db.Model(&model.Site{}).Where("id = ?", id).Updates(map[string]any{
		"icon_hash":   hash,
		"icon_type":   contentType,
		"icon_state":  model.IconStateReady,
		"icon_source": source,
	}).Error
}

// MarkIconFailed 标记图标抓取失败（图标退回内置占位图）。
func (s *Store) MarkIconFailed(id uint) error {
	return s.db.Model(&model.Site{}).Where("id = ?", id).Updates(map[string]any{
		"icon_state":  model.IconStateFailed,
		"icon_source": model.IconSourcePlaceholder,
	}).Error
}

// MarkIconPending 重新标记为待抓取（同时清掉旧来源，避免重抓期间用错底色）。
func (s *Store) MarkIconPending(id uint) error {
	return s.db.Model(&model.Site{}).Where("id = ?", id).Updates(map[string]any{
		"icon_hash":   "",
		"icon_type":   "",
		"icon_state":  model.IconStatePending,
		"icon_source": "",
	}).Error
}

// ---------------------------------------------------------------- 图标缓存

// SaveIconCache 写入/更新域名图标缓存记录。
func (s *Store) SaveIconCache(ic *model.IconCache) error {
	if ic.FetchedAt.IsZero() {
		ic.FetchedAt = time.Now().UTC()
	}
	return s.db.Save(ic).Error
}

// WriteIconFile 落盘图标文件（内容哈希命名，避免可枚举），返回文件路径。
func (s *Store) WriteIconFile(hash, ext string, data []byte) (string, error) {
	if hash == "" {
		return "", errors.New("empty hash")
	}
	if ext == "" {
		ext = ".img"
	}
	path := filepath.Join(s.iconsDir, hash+ext)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	// 临时文件名随机：两个站点并发写同一个图标时不会互相覆盖 / 抢走对方的临时文件
	tmp, err := os.CreateTemp(s.iconsDir, hash+".*.tmp")
	if err != nil {
		return "", err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o644)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
		return "", werr
	}
	return path, nil
}

// FindIconFile 按哈希查找已缓存的图标文件，返回路径与扩展名。
func (s *Store) FindIconFile(hash string) (string, bool) {
	if hash == "" {
		return "", false
	}
	matches, err := filepath.Glob(filepath.Join(s.iconsDir, hash+"*"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	for _, m := range matches {
		if strings.HasSuffix(m, ".tmp") {
			continue
		}
		if info, err := os.Stat(m); err == nil && info.Mode().IsRegular() {
			return m, true
		}
	}
	return "", false
}

// RemoveIconFiles 删除全部图标缓存文件。
func (s *Store) RemoveIconFiles() error {
	entries, err := os.ReadDir(s.iconsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if err := os.Remove(filepath.Join(s.iconsDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func moveInSlice(ids []uint, id uint, dir int) (bool, error) {
	idx := -1
	for i, v := range ids {
		if v == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, ErrNotFound
	}
	target := idx + dir
	if target < 0 || target >= len(ids) {
		return false, nil
	}
	ids[idx], ids[target] = ids[target], ids[idx]
	return true, nil
}
