// Command nasvia 启动 NASVIA 服务：单进程同时提供 API 与内嵌前端页面。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oner8/nasvia/internal/config"
	"github.com/oner8/nasvia/internal/model"
	"github.com/oner8/nasvia/internal/server"
	"github.com/oner8/nasvia/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[nasvia] ")

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("配置解析失败：%v", err)
	}

	st, err := store.Open(cfg.DBPath, cfg.IconsDir())
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}
	defer func() { _ = st.Close() }()

	if err := st.Migrate(); err != nil {
		log.Fatalf("初始化表结构失败：%v", err)
	}
	if err := st.EnsureDefaults(map[string]string{
		model.SettingAuthMode:       cfg.AuthMode,
		model.SettingLANCIDRs:       cfg.LANCIDRs,
		model.SettingHomeEgress:     cfg.HomeEgress,
		model.SettingSiteTitle:      cfg.SiteTitle,
		model.SettingFaviconSources: cfg.FaviconSources,
	}); err != nil {
		log.Fatalf("写入默认配置失败：%v", err)
	}
	// 一次性升级：库里还是旧版默认来源的（用户没手动改过）补上 nasicon 兜底来源。
	if upgraded, err := st.UpgradeFaviconSources(config.LegacyDefaultFaviconSources, cfg.FaviconSources); err != nil {
		log.Printf("升级图标来源设置失败：%v", err)
	} else if upgraded {
		log.Printf("图标来源已升级：%s", cfg.FaviconSources)
	}
	if seeded, err := st.Seed(); err != nil {
		log.Printf("写入演示数据失败：%v", err)
	} else if seeded {
		log.Printf("已写入演示站点与分类（可在后台删除或一键清空）")
	}

	srv, err := server.New(cfg, st)
	if err != nil {
		log.Fatalf("初始化服务失败：%v", err)
	}
	// 旧版本落盘的大图标（HD-Icons 原图 1024px）就地缩小，省流量、加快加载；已缩过的会被跳过。
	go func() {
		if n, err := srv.ShrinkStoredIcons(); err != nil {
			log.Printf("缩小图标缓存失败：%v", err)
		} else if n > 0 {
			log.Printf("已缩小 %d 个图标缓存文件", n)
		}
	}()

	authMode := st.Setting(model.SettingAuthMode, cfg.AuthMode)
	passwordSet := cfg.Password != ""
	if hash, err := st.PasswordHash(); err == nil && hash != "" {
		passwordSet = true
	}
	if !passwordSet {
		log.Printf("提示：未设置密码，后台管理接口将拒绝访问（仅可浏览公开内容）。请设置 NASVIA_PASSWORD。")
	}
	if authMode == model.AuthModePrivate && !passwordSet {
		log.Printf("警告：当前为「私密」模式但未设置密码，站点将无法访问；请设置 NASVIA_PASSWORD 或改回 public。")
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Printf("收到退出信号，正在关闭服务…")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("关闭服务出错：%v", err)
		}
	}()

	log.Printf("NASVIA %s 已启动：http://%s （数据目录 %s，数据库 %s）",
		config.Version, cfg.Addr(), cfg.DataDir, cfg.DBPath)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("服务异常退出：%v", err)
	}
	log.Printf("已停止")
}
