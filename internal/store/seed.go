package store

import (
	"time"

	"gorm.io/gorm"

	"github.com/oner8/nasvia/internal/model"
)

// Seed 首次启动（settings 中不存在 installed 键）时写入一套演示数据：
// 5 个分类、每个分类 10 个站点（共 50 个）——媒体中心 / 下载与整理 / 家庭自动化 /
// 网络与安全 四个公开分类，加一个「私密分类」私密分类（其下站点对匿名访客不可见），
// 用来演示分类分组、内外网切换、图标自动匹配与可见性过滤。
// 已清空过数据的实例不会再次被播种。
func (s *Store) Seed() (bool, error) {
	if s.IsInstalled() {
		return false, nil
	}

	cats := []model.Category{
		{Name: "媒体中心", Visibility: model.VisibilityPublic},
		{Name: "下载与整理", Visibility: model.VisibilityPublic},
		{Name: "家庭自动化", Visibility: model.VisibilityPublic},
		{Name: "网络与安全", Visibility: model.VisibilityPublic},
		{Name: "私密分类", Visibility: model.VisibilityPrivate},
	}

	sites := map[string][]model.Site{
		"媒体中心": {
			{Name: "Jellyfin", Description: "电影 / 剧集流媒体服务", URL: "https://jellyfin.home.example.com", LanURL: "http://192.168.1.10:8096", Visibility: model.VisibilityPublic, Pinned: true, Tags: "影音"},
			{Name: "Emby", Description: "另一套影视流媒体服务", URL: "https://emby.home.example.com", LanURL: "http://192.168.1.10:8920", Visibility: model.VisibilityPublic, Tags: "影音"},
			{Name: "Plex", Description: "Plex Media Server", URL: "https://plex.home.example.com", LanURL: "http://192.168.1.10:32400/web", Visibility: model.VisibilityPublic, Tags: "影音"},
			{Name: "Jellyseerr", Description: "影视请求与审批（对接 Jellyfin）", URL: "https://requests.home.example.com", LanURL: "http://192.168.1.10:5055", Visibility: model.VisibilityInherit, Tags: "请求"},
			{Name: "Tautulli", Description: "播放记录与观看统计", URL: "https://tautulli.home.example.com", LanURL: "http://192.168.1.10:8181", Visibility: model.VisibilityInherit, Tags: "统计"},
			{Name: "Navidrome", Description: "个人音乐流服务", URL: "https://music.home.example.com", LanURL: "http://192.168.1.10:4533", Visibility: model.VisibilityInherit, Tags: "音乐"},
			{Name: "Audiobookshelf", Description: "有声书与播客服务", URL: "https://audio.home.example.com", LanURL: "http://192.168.1.10:13378", Visibility: model.VisibilityInherit, Tags: "有声书"},
			{Name: "Calibre-Web", Description: "电子书库管理", URL: "https://books.home.example.com", LanURL: "http://192.168.1.10:8083", Visibility: model.VisibilityInherit, Tags: "电子书"},
			{Name: "Immich", Description: "自托管照片与视频备份", URL: "https://immich.home.example.com", LanURL: "http://192.168.1.10:2283", Visibility: model.VisibilityPublic, Tags: "照片"},
			{Name: "PhotoPrism", Description: "照片 AI 分类与浏览", URL: "https://photos.home.example.com", LanURL: "http://192.168.1.10:2342", Visibility: model.VisibilityInherit, Tags: "照片"},
		},
		"下载与整理": {
			{Name: "qBittorrent", Description: "BT / PT 下载客户端", URL: "https://qb.home.example.com", LanURL: "http://192.168.1.10:8080", Visibility: model.VisibilityPublic, Tags: "下载"},
			{Name: "Transmission", Description: "轻量 BT 下载客户端", URL: "https://transmission.home.example.com", LanURL: "http://192.168.1.10:9091", Visibility: model.VisibilityInherit, Tags: "下载"},
			{Name: "Deluge", Description: "插件式 BT 下载客户端", URL: "https://deluge.home.example.com", LanURL: "http://192.168.1.10:8112", Visibility: model.VisibilityInherit, Tags: "下载"},
			{Name: "Prowlarr", Description: "索引器统一管理（对接 Sonarr / Radarr）", URL: "https://prowlarr.home.example.com", LanURL: "http://192.168.1.10:9696", Visibility: model.VisibilityInherit, Tags: "索引"},
			{Name: "Sonarr", Description: "剧集自动下载与整理", URL: "https://sonarr.home.example.com", LanURL: "http://192.168.1.10:8989", Visibility: model.VisibilityInherit, Tags: "剧集"},
			{Name: "Radarr", Description: "电影自动下载与整理", URL: "https://radarr.home.example.com", LanURL: "http://192.168.1.10:7878", Visibility: model.VisibilityInherit, Tags: "电影"},
			{Name: "Readarr", Description: "电子书自动下载与整理", URL: "https://readarr.home.example.com", LanURL: "http://192.168.1.10:8787", Visibility: model.VisibilityInherit, Tags: "图书"},
			{Name: "Bazarr", Description: "字幕自动匹配与下载", URL: "https://bazarr.home.example.com", LanURL: "http://192.168.1.10:6767", Visibility: model.VisibilityInherit, Tags: "字幕"},
			{Name: "Autobrr", Description: "种子发布自动抢种", URL: "https://autobrr.home.example.com", LanURL: "http://192.168.1.10:7474", Visibility: model.VisibilityInherit, Tags: "自动化"},
			{Name: "MoviePilot", Description: "媒体库自动化流水线", URL: "https://moviepilot.home.example.com", LanURL: "http://192.168.1.10:8010", Visibility: model.VisibilityInherit, Tags: "自动化"},
		},
		"家庭自动化": {
			{Name: "Home Assistant", Description: "智能家居中枢", URL: "https://ha.home.example.com", LanURL: "http://192.168.1.10:8123", Visibility: model.VisibilityInherit, Tags: "智能家居"},
			{Name: "ESPHome", Description: "ESP 设备固件管理", URL: "https://esphome.home.example.com", LanURL: "http://192.168.1.10:6052", Visibility: model.VisibilityInherit, Tags: "固件"},
			{Name: "Node-RED", Description: "可视化流程编排", URL: "https://nodered.home.example.com", LanURL: "http://192.168.1.10:1880", Visibility: model.VisibilityPublic, Tags: "流程"},
			{Name: "Frigate", Description: "摄像头与 AI 事件检测", URL: "https://frigate.home.example.com", LanURL: "http://192.168.1.10:8971", Visibility: model.VisibilityInherit, Tags: "监控"},
			{Name: "Zigbee2MQTT", Description: "Zigbee 网关桥接", URL: "https://zigbee.home.example.com", LanURL: "http://192.168.1.10:8089", Visibility: model.VisibilityInherit, Tags: "网关"},
			{Name: "Homebridge", Description: "HomeKit 设备桥接", URL: "https://homebridge.home.example.com", LanURL: "http://192.168.1.10:8581", Visibility: model.VisibilityInherit, Tags: "桥接"},
			{Name: "Mosquitto", Description: "MQTT 消息代理", URL: "https://mqtt.home.example.com", LanURL: "http://192.168.1.10:1883", Visibility: model.VisibilityInherit, Tags: "MQTT"},
			{Name: "InfluxDB", Description: "传感器时序数据库", URL: "https://influx.home.example.com", LanURL: "http://192.168.1.10:8086", Visibility: model.VisibilityInherit, Tags: "数据库"},
			{Name: "Mijia", Description: "米家 · 小米智能家居", URL: "https://home.mi.com", Visibility: model.VisibilityInherit, Tags: "智能家居"},
			{Name: "Tasmota", Description: "刷机设备控制台", URL: "https://tasmota.home.example.com", LanURL: "http://192.168.1.10:8099", Visibility: model.VisibilityInherit, Tags: "固件"},
		},
		"网络与安全": {
			{Name: "AdGuard Home", Description: "全网 DNS 广告过滤", URL: "https://adguard.home.example.com", LanURL: "http://192.168.1.10:3001", Visibility: model.VisibilityPublic, Pinned: true, Tags: "DNS"},
			{Name: "Pi-hole", Description: "DNS 黑洞与广告过滤", URL: "https://pihole.home.example.com", LanURL: "http://192.168.1.10:8053", Visibility: model.VisibilityInherit, Tags: "DNS"},
			{Name: "Nginx Proxy Manager", Description: "反向代理与证书管理", URL: "https://npm.home.example.com", LanURL: "http://192.168.1.10:81", Visibility: model.VisibilityPublic, Tags: "反代"},
			{Name: "Traefik", Description: "云原生反向代理", URL: "https://traefik.home.example.com", LanURL: "http://192.168.1.10:8088", Visibility: model.VisibilityInherit, Tags: "反代"},
			{Name: "WireGuard", Description: "远程接入 VPN", URL: "https://vpn.home.example.com", LanURL: "http://192.168.1.10:51821", Visibility: model.VisibilityInherit, Tags: "VPN"},
			{Name: "Tailscale", Description: "异地组网（仅外网控制台）", URL: "https://login.tailscale.com/admin/machines", Visibility: model.VisibilityPublic, Tags: "组网"},
			{Name: "Authelia", Description: "单点登录与二次验证", URL: "https://auth.home.example.com", LanURL: "http://192.168.1.10:9091", Visibility: model.VisibilityInherit, Tags: "认证"},
			{Name: "Netdata", Description: "主机实时性能监控", URL: "https://netdata.home.example.com", LanURL: "http://192.168.1.10:19999", Visibility: model.VisibilityInherit, Tags: "监控"},
			{Name: "LibreSpeed", Description: "自托管内网测速", URL: "https://speed.home.example.com", LanURL: "http://192.168.1.10:8095", Visibility: model.VisibilityInherit, Tags: "测速"},
			{Name: "ddns-updater", Description: "动态域名解析更新", URL: "https://ddns.home.example.com", LanURL: "http://192.168.1.10:8097", Visibility: model.VisibilityInherit, Tags: "DNS"},
		},
		"私密分类": {
			{Name: "Vaultwarden", Description: "自托管密码库（仅自己可见）", URL: "https://vault.home.example.com", LanURL: "http://192.168.1.10:8081", Visibility: model.VisibilityPrivate, Tags: "密码"},
			{Name: "群晖 DSM", Description: "NAS 管理后台", URL: "https://dsm.home.example.com", LanURL: "http://192.168.1.10:5001", Visibility: model.VisibilityPrivate, Pinned: true, Tags: "NAS"},
			{Name: "OpenWrt 路由", Description: "主路由管理界面", URL: "https://router.home.example.com", LanURL: "http://192.168.1.1", Visibility: model.VisibilityPrivate, Tags: "路由"},
			{Name: "File Station", Description: "NAS 文件管理（继承分类可见性）", URL: "https://files.home.example.com", LanURL: "http://192.168.1.10:5002", Visibility: model.VisibilityInherit, Tags: "文件"},
			{Name: "Syncthing", Description: "多设备文件同步（继承分类可见性）", URL: "https://sync.home.example.com", LanURL: "http://192.168.1.10:8384", Visibility: model.VisibilityInherit, Tags: "同步"},
			{Name: "Nextcloud", Description: "私有云盘与在线协作", URL: "https://cloud.home.example.com", LanURL: "http://192.168.1.10:8082", Visibility: model.VisibilityInherit, Tags: "云盘"},
			{Name: "Filebrowser", Description: "轻量网页文件管理器", URL: "https://browse.home.example.com", LanURL: "http://192.168.1.10:8084", Visibility: model.VisibilityInherit, Tags: "文件"},
			{Name: "Seafile", Description: "文件同步与共享", URL: "https://seafile.home.example.com", LanURL: "http://192.168.1.10:8098", Visibility: model.VisibilityInherit, Tags: "同步"},
			{Name: "Duplicati", Description: "加密增量备份", URL: "https://backup.home.example.com", LanURL: "http://192.168.1.10:8200", Visibility: model.VisibilityInherit, Tags: "备份"},
			{Name: "Paperless-ngx", Description: "纸质文档数字化归档", URL: "https://paperless.home.example.com", LanURL: "http://192.168.1.10:8000", Visibility: model.VisibilityInherit, Tags: "文档"},
		},
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		sort := 0
		for _, cat := range cats {
			category := cat
			if err := tx.Create(&category).Error; err != nil {
				return err
			}
			for _, site := range sites[category.Name] {
				sort++
				item := site
				item.Sort = sort
				item.CategoryID = &category.ID
				if item.Visibility == "" {
					item.Visibility = model.VisibilityInherit
				}
				item.IconState = model.IconStatePending
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
		}
		return tx.Save(&model.Setting{
			Key:   model.SettingInstalled,
			Value: time.Now().UTC().Format(time.RFC3339),
		}).Error
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
