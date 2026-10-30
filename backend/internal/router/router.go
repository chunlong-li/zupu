package router

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"familytree/internal/backup"
	"familytree/internal/config"
	"familytree/internal/handler"

	"github.com/gin-gonic/gin"
)

// Setup 初始化路由
func Setup(cfg *config.Config, db *sql.DB, scheduler *backup.Scheduler) *gin.Engine {
	r := gin.Default()
	// 在线更新包可能较大，放宽 multipart 内存缓存到 512MB（超过则落临时文件）
	r.MaxMultipartMemory = 512 << 20

	// 静态资源：优先读更新包部署的 /data/runtime/web，其次 /data/web（手动部署），最后内置 ./web
	webDir := cfg.WebDir
	if webDir == "" {
		webDir = "./web"
	}
	if stat, err := os.Stat(webDir); err != nil || !stat.IsDir() {
		// 兼容旧部署路径；安装包通过 FAMILY_TREE_WEB_DIR 指向 ${TRIM_APPDEST}/web
		if stat, err := os.Stat("/data/runtime/web"); err == nil && stat.IsDir() {
			webDir = "/data/runtime/web"
		} else if stat, err := os.Stat("/data/web"); err == nil && stat.IsDir() {
			webDir = "/data/web"
		}
	}
	log.Printf("[web] root=%s", webDir)
	// 静态资源缓存策略（治「更新后浏览器还在跑旧代码」）：
	//   /assets/*：Vite 产物文件名自带内容哈希 → 可长期强缓存
	//   index.html：必须每次回源校验，否则更新后浏览器仍用旧 HTML 里的旧 chunk
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	})
	r.Static("/assets", webDir+"/assets")
	// 功能测试：zaxchou/zupu 开源族谱工具（单文件静态页）
	r.Static("/zupu-test", webDir+"/zupu-test")
	r.Static("/uploads", cfg.DataDir+"/upload")
	os.MkdirAll(filepath.Join(cfg.DataDir, "fonts"), 0o755)
	r.Static("/fonts", cfg.DataDir+"/fonts")
	os.MkdirAll(filepath.Join(cfg.DataDir, "map"), 0o755)
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		log.Printf("[spa] host=%s path=%s -> index.html", c.Request.Host, c.Request.URL.Path)
		c.File(webDir + "/index.html")
	})

	thumb := &handler.ThumbnailHandler{DB: db, DataDir: cfg.DataDir}
	r.GET("/api/thumbnail", thumb.ServeThumbnail)
	// 离线地图包分发（无需登录：供其他家族系统从本机 map 目录拉包）
	mp := &handler.MapPackHandler{DB: db, DataDir: cfg.DataDir, Cfg: cfg}
	mp.EnsureStart() // 启动额度用尽后的自动续传检查（每 5 分钟，无需打开页面）
	// 生僻字查字数据（笔画/部首/拼音，约 1.9MB JSON）
	r.StaticFile("/hanzi-data.json", webDir+"/hanzi-data.json")
	r.NoRoute(func(c *gin.Context) {
		// API 未知路径必须返回 JSON 404，不能回 index.html
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// 前端静态目录下真实存在的文件（地图 GeoJSON、图标等）直接返回；
		// 其余路径交回 SPA（index.html），由前端路由处理
		if p := c.Request.URL.Path; p != "/" {
			clean := filepath.Clean(p)
			if !strings.Contains(clean, "..") {
				fp := filepath.Join(webDir, clean)
				if st, err := os.Stat(fp); err == nil && !st.IsDir() {
					c.File(fp)
					return
				}
			}
		}
		// 静态资源确实不存在时返回真 404：不能拿 index.html 兜底，
		// 否则浏览器请求 .js/.css 会拿到 HTML（MIME 不符），表现为「更新后仍在跑旧代码」
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			// 覆盖上面的长缓存头：404 绝不能带 immutable，否则更新过程中
			// 短暂缺文件会被浏览器缓存一年，页面从此报错
			c.Header("Cache-Control", "no-store")
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-cache, must-revalidate")
		log.Printf("[spa] host=%s path=%s -> index.html", c.Request.Host, c.Request.URL.Path)
		c.File(webDir + "/index.html")
	})

	// 传承服务器（家族知识库）反代：/knowledge/* → 传承服务器（注入 SSO 身份）
	kbH := &handler.KBHandler{DB: db, Cfg: cfg}
	r.GET("/knowledge", func(c *gin.Context) { c.Redirect(http.StatusFound, "/knowledge/?"+c.Request.URL.RawQuery) })
	r.Any("/knowledge/*path", kbH.Proxy)

	api := r.Group("/api")
	{
		// 认证（无需登录）
		auth := &handler.AuthHandler{DB: db}
		api.POST("/auth/login", auth.Login)
		api.POST("/auth/verify", auth.Verify) // 仅供传承服务器校验族谱账号（不签发 token）
		api.GET("/auth/check", auth.CheckAuth)

		// 初始化加密密钥（首次启动自动生成）
		encKey, _ := handler.GetOrCreateEncKey(db)

		// 公开接口（无需登录，供数据看板统计用）
		memberPublic := &handler.MemberHandler{DB: db, EncKey: encKey, DataDir: cfg.DataDir}
		api.GET("/members/stats", memberPublic.Stats)

		// 公开的家族设置读取接口（无需登录）
		settingsPublic := &handler.SettingsHandler{DB: db}
		api.GET("/settings/family-name", settingsPublic.GetFamilyName)
		api.GET("/settings/family-surname", settingsPublic.GetFamilySurname)
		api.GET("/settings/version", settingsPublic.GetVersion)
		api.GET("/settings/role-aliases", settingsPublic.GetRoleAliases)

		// 授权接口（公开：查询状态和实例ID；激活需登录）

		// 传承服务器（家族知识库）只读同步导出（共享密钥鉴权，无需登录）
		kbExp := &handler.KBExportHandler{DB: db, Secret: cfg.KBSSOSecret}
		api.GET("/kb/export", kbExp.Export)

		// 离线地图包分发（无需登录：供其他家族系统从本机 map 目录拉包）

		// 需要登录的路由
		authGroup := api.Group("")
		authGroup.Use(handler.AuthMiddleware(db))
		authGroup.Use(handler.AuditMiddleware(db)) // 操作日志采集（记录 POST/PUT/DELETE 写操作）
		{
			authGroup.POST("/auth/logout", auth.Logout)
			authGroup.POST("/auth/change-password", auth.ChangePassword)

			// 传承服务器（家族知识库）状态探测
			authGroup.GET("/kb/status", handler.RequirePriv("kb"), kbH.Status)

			// 用户管理：权限由「隐私与权限」矩阵控制
			authGroup.GET("/users", handler.RequirePriv("account_view"), auth.ListUsers)
			authGroup.POST("/users", handler.RequirePriv("account_create"), auth.CreateUser)
			authGroup.PUT("/users/:id/role", handler.RequirePriv("account_manage"), auth.SetRole)
			authGroup.PUT("/users/:id/branch", handler.RequirePriv("account_manage"), auth.SetBranch)
			authGroup.POST("/users/:id/reset-password", handler.RequirePriv("account_manage"), auth.ResetPassword)
			authGroup.DELETE("/users/:id", handler.RequirePriv("account_delete"), auth.DeleteUser)

			// 成员管理（查询：族员；增改删：理事及以上，受人数上限限制）
			member := &handler.MemberHandler{DB: db, EncKey: encKey, DataDir: cfg.DataDir}
			authGroup.GET("/members", handler.RequireRole(handler.RoleMember), member.List)
			authGroup.GET("/members/with-spouses", handler.RequireRole(handler.RoleMember), member.ListWithSpouses)
			authGroup.GET("/members/chuizhu", handler.RequireRole(handler.RoleMember), member.ListChuiZhu)
			authGroup.GET("/members/search", handler.RequireRole(handler.RoleMember), member.Search)
			authGroup.GET("/members/:id", handler.RequireRole(handler.RoleMember), member.Get)
			authGroup.GET("/members/:id/detail", handler.RequireRole(handler.RoleMember), member.GetWithSpouses)
			authGroup.GET("/members/:id/children", handler.RequireRole(handler.RoleMember), member.Children)
			authGroup.GET("/members/:id/spouses-for-mother", handler.RequireRole(handler.RoleMember), member.GetSpousesAsMothers)
			authGroup.GET("/members/:id/dna-match", handler.RequireRole(handler.RoleChief), member.DNAMatch)
			authGroup.POST("/members/dna-str-compare", handler.RequireRole(handler.RoleChief), member.CompareDNASTR)
			authGroup.POST("/members/dna-match-external", handler.RequireRole(handler.RoleChief), member.DNAMatchExternal)
			// 写操作：已取消人数上限限制（免费版改为导出水印，2026-09-13）
			authGroup.POST("/members", handler.RequirePriv("member_edit"), member.Create)
			authGroup.PUT("/members/:id", handler.RequirePriv("member_edit"), member.Update)
			authGroup.PUT("/members/:id/rename", handler.RequirePriv("member_edit"), member.Rename)
			authGroup.PUT("/members/:id/flags", handler.RequirePriv("member_edit"), member.UpdateFlags)
			authGroup.DELETE("/members/:id", handler.RequirePriv("member_edit"), member.Delete)

			// 成员档案 Excel/CSV 导入导出（导出/模板：export 权限；导入：理事长及以上）
			// 数据体检 GET /members/check（设置 → 族谱数据 分组；理事长及以上）
			memberIO := &handler.MemberIOHandler{DB: db, EncKey: encKey, DataDir: cfg.DataDir}
			authGroup.GET("/members/export", handler.RequirePriv("export"), memberIO.Export)
			authGroup.GET("/members/import/template", handler.RequirePriv("export"), memberIO.Template)
			authGroup.POST("/members/import/preview", handler.RequireRole(handler.RoleChief), memberIO.Preview)
			authGroup.POST("/members/import/failures", handler.RequireRole(handler.RoleChief), memberIO.FailuresCSV)
			authGroup.POST("/members/import", handler.RequireRole(handler.RoleChief), memberIO.Import)
			authGroup.GET("/members/check", handler.RequireRole(handler.RoleChief), memberIO.Check)

			// 关系计算（族员）
			authGroup.GET("/members/:id/relations", handler.RequireRole(handler.RoleMember), member.Relations)

			// 附件上传（族员查看，理事上传/删除）
			upload := &handler.UploadHandler{DB: db, Cfg: cfg}
			media := &handler.MediaHandler{DB: db, Cfg: cfg}
			authGroup.POST("/upload", handler.RequirePriv("member_edit"), upload.Upload)
			authGroup.GET("/members/:id/attachments", handler.RequireRole(handler.RoleMember), upload.ListByMember)
			authGroup.GET("/spouses/:id/attachments", handler.RequireRole(handler.RoleMember), upload.ListBySpouse)
			authGroup.DELETE("/attachments/:id", handler.RequirePriv("member_edit"), upload.Delete)
			// 通用多媒体库（祠堂照片 / 传统节日图片视频 / 其他族务影像）
			authGroup.GET("/media", handler.RequireRole(handler.RoleMember), media.List)
			authGroup.GET("/media/categories", handler.RequireRole(handler.RoleMember), media.Categories)
			authGroup.POST("/media/upload", handler.RequirePriv("member_edit"), media.Upload)
			authGroup.PUT("/media/:id", handler.RequirePriv("member_edit"), media.Update)
			authGroup.DELETE("/media/:id", handler.RequirePriv("member_edit"), media.Delete)
			// 孤儿文件（upload/ 里既不被 media_assets 也不被 attachments / 头像引用）：先列清单再清
			authGroup.GET("/media/orphans", handler.RequirePriv("member_edit"), media.Orphans)
			authGroup.POST("/media/orphans/clean", handler.RequirePriv("member_edit"), media.CleanOrphans)

			// 谱书自定义字体（系统级保存，换浏览器/设备可用）：族员可读，理事长可传/删
			fontH := &handler.FontHandler{Cfg: cfg}
			authGroup.GET("/fonts", handler.RequireRole(handler.RoleMember), fontH.List)
			authGroup.POST("/fonts", handler.RequireRole(handler.RoleChief), fontH.Upload)
			authGroup.DELETE("/fonts/:name", handler.RequireRole(handler.RoleChief), fontH.Delete)

			// 生平条目（查询：族员；增改删：理事及以上）
			bio := &handler.BioHandler{DB: db}
			authGroup.GET("/members/:id/bio-entries", handler.RequireRole(handler.RoleMember), bio.ListByMember)
			authGroup.POST("/members/:id/bio-entries", handler.RequirePriv("member_edit"), bio.Create)
			authGroup.PUT("/bio-entries/:id", handler.RequirePriv("member_edit"), bio.Update)
			authGroup.DELETE("/bio-entries/:id", handler.RequirePriv("member_edit"), bio.Delete)

			// 资料来源（查询：族员；增删：成员编辑权限 member_edit，含房支隔离）
			// 谱书三版式的行传页要用「全量」做「据：…」小字脚注 → 单列一个全量接口，不改成员接口
			src := &handler.SourceHandler{DB: db}
			authGroup.GET("/member-sources", handler.RequireRole(handler.RoleMember), src.ListAll)
			authGroup.GET("/members/:id/sources", handler.RequireRole(handler.RoleMember), src.ListByMember)
			authGroup.POST("/members/:id/sources", handler.RequirePriv("member_edit"), src.Create)
			authGroup.DELETE("/sources/:id", handler.RequirePriv("member_edit"), src.Delete)

			// 配偶生平条目（查询：族员；增改删：理事及以上）
			authGroup.GET("/spouses/:id/bio-entries", handler.RequireRole(handler.RoleMember), bio.ListBySpouse)
			authGroup.POST("/spouses/:id/bio-entries", handler.RequirePriv("member_edit"), bio.CreateSpouse)
			authGroup.PUT("/spouse-bio-entries/:id", handler.RequirePriv("member_edit"), bio.UpdateSpouse)
			authGroup.DELETE("/spouse-bio-entries/:id", handler.RequirePriv("member_edit"), bio.DeleteSpouse)

			// 配偶管理（查询：族员；增改删：理事）
			spouse := &handler.SpouseHandler{DB: db, EncKey: encKey}
			authGroup.GET("/members/:id/spouses", handler.RequireRole(handler.RoleMember), spouse.ListByMember)
			authGroup.GET("/spouses", handler.RequireRole(handler.RoleMember), spouse.GetAll)
			authGroup.GET("/spouses/:id", handler.RequireRole(handler.RoleMember), spouse.Get)
			authGroup.POST("/spouses", handler.RequirePriv("member_edit"), spouse.Create)
			authGroup.PUT("/spouses/:id", handler.RequirePriv("member_edit"), spouse.Update)
			authGroup.DELETE("/spouses/:id", handler.RequirePriv("member_edit"), spouse.Delete)

			// 版本历史 / 回滚（查看：族员；回滚：仅理事长与超级管理员）
			verlog := &handler.VersionLogHandler{DB: db}
			authGroup.GET("/members/:id/versions", handler.RequireRole(handler.RoleMember), verlog.ListMemberVersions)
			authGroup.GET("/spouses/:id/versions", handler.RequireRole(handler.RoleMember), verlog.ListSpouseVersions)
			authGroup.GET("/entity-versions/:id/changes", handler.RequireRole(handler.RoleMember), verlog.VersionChanges)
			authGroup.POST("/entity-versions/:id/rollback", handler.RequireRole(handler.RoleChief), verlog.Rollback)

			// 提交-审核（草稿队列）：查看与审核仅理事长及以上
			draft := &handler.DraftHandler{DB: db, EncKey: encKey}
			authGroup.GET("/drafts", handler.RequirePriv("review"), draft.ListDrafts)
			authGroup.POST("/drafts/:id/review", handler.RequirePriv("review"), draft.ReviewDraft)

			// 日期换算（族员）
			lunar := &handler.LunarHandler{DB: db}
			authGroup.GET("/lunar/convert", handler.RequireRole(handler.RoleMember), lunar.Convert)
			authGroup.GET("/lunar/to-solar", handler.RequireRole(handler.RoleMember), lunar.LunarToSolar)

			// 字辈派语（查询：族员；增改删：理事）
			gw := &handler.GenerationWordHandler{DB: db}
			authGroup.GET("/generation-words", handler.RequireRole(handler.RoleMember), gw.List)
			authGroup.POST("/generation-words", handler.RequirePriv("member_edit"), gw.Create)
			authGroup.PUT("/generation-words/:id", handler.RequirePriv("member_edit"), gw.Update)
			authGroup.DELETE("/generation-words/:id", handler.RequirePriv("member_edit"), gw.Delete)

			// 房支（宗族→房支→成员三级分类：查询族员，增改删/分配 理事及以上）
			branch := &handler.BranchHandler{DB: db}
			authGroup.GET("/branches", handler.RequireRole(handler.RoleMember), branch.List)
			authGroup.POST("/branches", handler.RequirePriv("member_edit"), branch.Create)
			authGroup.PUT("/branches/:id", handler.RequirePriv("member_edit"), branch.Update)
			authGroup.DELETE("/branches/:id", handler.RequirePriv("member_edit"), branch.Delete)
			authGroup.POST("/branches/set-members", handler.RequirePriv("member_edit"), branch.SetMemberBranch)
			authGroup.POST("/branches/backfill", handler.RequireRole(handler.RoleChief), branch.Backfill)

			// 家族大事记 / 谱序 / 祠墓（查询：族员；增改删：理事）
			ev := &handler.EventHandler{DB: db}
			authGroup.GET("/events", handler.RequireRole(handler.RoleMember), ev.ListEvents)
			authGroup.POST("/events", handler.RequirePriv("member_edit"), ev.CreateEvent)
			authGroup.PUT("/events/:id", handler.RequirePriv("member_edit"), ev.UpdateEvent)
			authGroup.DELETE("/events/:id", handler.RequirePriv("member_edit"), ev.DeleteEvent)

			authGroup.GET("/articles", handler.RequireRole(handler.RoleMember), ev.ListArticles)
			authGroup.POST("/articles", handler.RequirePriv("member_edit"), ev.CreateArticle)
			authGroup.PUT("/articles/:id", handler.RequirePriv("member_edit"), ev.UpdateArticle)
			authGroup.DELETE("/articles/:id", handler.RequirePriv("member_edit"), ev.DeleteArticle)

			authGroup.GET("/cemeteries", handler.RequireRole(handler.RoleMember), ev.ListCemeteries)
			authGroup.POST("/cemeteries", handler.RequirePriv("member_edit"), ev.CreateCemetery)
			authGroup.PUT("/cemeteries/:id", handler.RequirePriv("member_edit"), ev.UpdateCemetery)
			authGroup.DELETE("/cemeteries/:id", handler.RequirePriv("member_edit"), ev.DeleteCemetery)

			// 祠宇志 / 茔墓志（真实祠堂、真实祖茔的档案；与「电子祠堂」/hall 是两回事）
			// 数据落在既有 cemeteries（kind='shrine'/'grave'）+ 3 张祠宇子表；
			// 查询：族员；增改删：member_edit（与祠墓/房支/字辈同口径，直写不进草稿队列）
			sg := &handler.ShrineGraveHandler{DB: db}
			authGroup.GET("/shrines", handler.RequireRole(handler.RoleMember), sg.ListShrines)
			authGroup.GET("/shrines/:id", handler.RequireRole(handler.RoleMember), sg.GetShrine)
			authGroup.POST("/shrines", handler.RequirePriv("member_edit"), sg.CreateShrine)
			authGroup.PUT("/shrines/:id", handler.RequirePriv("member_edit"), sg.UpdateShrine)
			authGroup.DELETE("/shrines/:id", handler.RequirePriv("member_edit"), sg.DeleteShrine)
			authGroup.GET("/graves", handler.RequireRole(handler.RoleMember), sg.ListGraves)
			authGroup.GET("/graves/:id", handler.RequireRole(handler.RoleMember), sg.GetGrave)
			authGroup.POST("/graves", handler.RequirePriv("member_edit"), sg.CreateGrave)
			authGroup.PUT("/graves/:id", handler.RequirePriv("member_edit"), sg.UpdateGrave)
			authGroup.DELETE("/graves/:id", handler.RequirePriv("member_edit"), sg.DeleteGrave)
			// 旧「祠墓」记录归入新模块（旧行 kind 为空，先读再归类；不删数据）
			authGroup.POST("/cemeteries/:id/classify", handler.RequirePriv("member_edit"), sg.ClassifyLegacy)

			// 名人传记（查询：族员；入传标准设置：理事长/超管；增改删：理事提交审核、理事长/超管直接发布）
			biog := &handler.BiographyHandler{DB: db}
			authGroup.GET("/biographies", handler.RequireRole(handler.RoleMember), biog.ListBiographies)
			authGroup.POST("/biographies", handler.RequirePriv("member_edit"), biog.CreateBiography)
			authGroup.GET("/biography-criteria", handler.RequireRole(handler.RoleMember), biog.GetBiographyCriteria)
			authGroup.PUT("/biography-criteria", handler.RequireRole(handler.RoleChief), biog.UpdateBiographyCriteria)
			authGroup.GET("/biographies/:id", handler.RequireRole(handler.RoleMember), biog.GetBiography)
			authGroup.PUT("/biographies/:id", handler.RequirePriv("member_edit"), biog.UpdateBiography)
			authGroup.DELETE("/biographies/:id", handler.RequirePriv("member_edit"), biog.DeleteBiography)

			// 祠堂上香（族员及以上）：记录上香人（留痕不显示），一炷香后香烧完删除；同一人香未烧完不能重复上
			incense := &handler.IncenseHandler{DB: db}
			authGroup.POST("/incense/offer", handler.RequireRole(handler.RoleMember), incense.Offer)
			authGroup.GET("/incense/active", handler.RequireRole(handler.RoleMember), incense.List)

			// 日历（生日/祭日，族员及以上）
			calendar := &handler.CalendarHandler{DB: db}
			authGroup.GET("/members/birthdays", handler.RequireRole(handler.RoleMember), calendar.Birthdays)
			authGroup.GET("/members/deathdays", handler.RequireRole(handler.RoleMember), calendar.Deathdays)
			// 传统日程（族员及以上可查看，理事及以上可编辑）
			trad := &handler.TraditionHandler{DB: db}
			authGroup.GET("/traditions", handler.RequireRole(handler.RoleMember), trad.ListTraditions)
			authGroup.POST("/traditions", handler.RequirePriv("member_edit"), trad.CreateTradition)
			authGroup.PUT("/traditions/:id", handler.RequirePriv("member_edit"), trad.UpdateTradition)
			authGroup.DELETE("/traditions/:id", handler.RequirePriv("member_edit"), trad.DeleteTradition)
			// 「不固定」节日的本年临时日期（按 节日 + 年份 存，过期自动失效）
			authGroup.PUT("/traditions/:id/temp-date", handler.RequirePriv("member_edit"), trad.SetTraditionTempDate)

			// 首页「未来 30 天」：传统节日 + 族人生日 + 忌日（族员及以上；游客/未登录 401）
			upcoming := &handler.UpcomingHandler{DB: db}
			authGroup.GET("/dashboard/upcoming", handler.RequireRole(handler.RoleMember), upcoming.GetUpcoming)

			// 家族热力分布（族员及以上）
			heat := &handler.HeatmapHandler{DB: db}
			authGroup.GET("/heatmap", handler.RequireRole(handler.RoleMember), heat.GetHeatmap)
			authGroup.GET("/heatmap/members", handler.RequireRole(handler.RoleMember), heat.GetHeatmapMembers)

			// 氏族迁徙记录（族员及以上可查看，理事及以上可编辑）
			mig := &handler.MigrationEventHandler{DB: db}
			authGroup.GET("/migrations", handler.RequireRole(handler.RoleMember), mig.List)
			authGroup.POST("/migrations", handler.RequirePriv("member_edit"), mig.Create)
			authGroup.PUT("/migrations/:id", handler.RequirePriv("member_edit"), mig.Update)
			authGroup.DELETE("/migrations/:id", handler.RequirePriv("member_edit"), mig.Delete)

			// 地理编码（百度/高德，Key 由用户自行填写；理事长及以上）
			geo := &handler.GeocodeHandler{DB: db}
			authGroup.GET("/heatmap/points", handler.RequireRole(handler.RoleMember), geo.Points)
			authGroup.GET("/geo/config", handler.RequireRole(handler.RoleChief), geo.GetConfig)
			authGroup.PUT("/geo/config", handler.RequireRole(handler.RoleChief), geo.SaveConfig)
			authGroup.DELETE("/geo/cache", handler.RequireRole(handler.RoleChief), geo.ClearCache)

			// 离线地图包（下载/管理，理事长及以上；瓦片读取族员及以上）
			authGroup.GET("/map/packs", handler.RequireRole(handler.RoleChief), mp.GetPacks)
			authGroup.POST("/map/import-pack", handler.RequireRole(handler.RoleChief), mp.ImportPack)
			authGroup.DELETE("/map/packs/:id", handler.RequireRole(handler.RoleChief), mp.DeletePack)
			authGroup.GET("/map/config", handler.RequireRole(handler.RoleChief), mp.GetConfig)
			authGroup.PUT("/map/config", handler.RequireRole(handler.RoleChief), mp.SaveConfig)
			authGroup.GET("/map/online-config", handler.RequireRole(handler.RoleMember), mp.GetOnlineConfig)
			authGroup.PUT("/map/online-config", handler.RequireRole(handler.RoleChief), mp.SaveOnlineConfig)
			authGroup.GET("/map/tiles/:pack/:layer/:z/:x/:y", handler.RequireRole(handler.RoleMember), mp.ServeTile)
			authGroup.GET("/map/online/:layer/:z/:x/:y", handler.RequireRole(handler.RoleMember), mp.ServeOnlineTile)

			// GEDCOM 导入导出（理事长及以上）
			ged := &handler.GedcomHandler{DB: db}
			authGroup.POST("/gedcom/import", handler.RequireRole(handler.RoleChief), ged.Import)
			authGroup.GET("/gedcom/export", handler.RequirePriv("export"), ged.Export)

			// 备份/还原（理事长及以上）
			backupHandler := &handler.BackupHandler{DB: db, Cfg: cfg}
			authGroup.POST("/backup", handler.RequirePriv("backup"), backupHandler.Backup)
			authGroup.GET("/backup/list", handler.RequirePriv("backup"), backupHandler.List)
			authGroup.GET("/backup/download", handler.RequirePriv("backup"), backupHandler.Download)
			authGroup.POST("/backup/delete", handler.RequirePriv("backup"), backupHandler.DeleteBackup)
			authGroup.POST("/backup/reset", handler.RequirePriv("backup"), backupHandler.Reset)
			authGroup.POST("/backup/system", handler.RequirePriv("backup"), backupHandler.SystemBackup)
			authGroup.POST("/backup/gedcom", handler.RequirePriv("backup"), backupHandler.BackupGedcom)
			authGroup.POST("/backup/gedcom-accounts", handler.RequirePriv("backup"), backupHandler.BackupGedcomAccounts)
			authGroup.POST("/restore", handler.RequirePriv("backup"), backupHandler.Restore)
			authGroup.POST("/restore/upload", handler.RequirePriv("backup"), backupHandler.UploadRestore)

			// 备份状态（族员可查看）
			authGroup.GET("/backup/status", handler.RequireRole(handler.RoleMember), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"auto_backup": true,
					"daily_keep":  30,
					"schedule":    "每日 02:00（全量+GEDCOM+多媒体快照，各保留30天）/ 每月1日 03:00",
				})
			})

			// 系统设置（修改：理事长及以上）
			settings := &handler.SettingsHandler{DB: db}
			authGroup.PUT("/settings/family-name", handler.RequirePriv("system_settings"), settings.UpdateFamilyName)
			authGroup.PUT("/settings/family-surname", handler.RequirePriv("system_settings"), settings.UpdateFamilySurname)
			authGroup.PUT("/settings/role-aliases", handler.RequirePriv("system_settings"), settings.UpdateRoleAliases)
			// 隐私与权限矩阵（读取/修改：理事长及以上）
			authGroup.GET("/settings/privacy-matrix", handler.RequireRole(handler.RoleChief), settings.GetPrivacyMatrix)
			authGroup.PUT("/settings/privacy-matrix", handler.RequireRole(handler.RoleChief), settings.UpdatePrivacyMatrix)
			// 登录防爆破配置（读取/修改：理事长及以上，即族长与超级管理员）
			authGroup.GET("/settings/login-security", handler.RequirePriv("system_settings"), settings.GetLoginSecurity)
			// 本机（127.0.0.1）访问免登录：默认关；仅理事长/超管（system_settings 权限项）可开关
			authGroup.GET("/settings/local-no-auth", handler.RequirePriv("system_settings"), settings.GetLocalNoAuth)
			authGroup.PUT("/settings/local-no-auth", handler.RequirePriv("system_settings"), settings.UpdateLocalNoAuth)
			authGroup.PUT("/settings/login-security", handler.RequirePriv("system_settings"), settings.UpdateLoginSecurity)
			authGroup.GET("/settings/demo-mode", handler.RequireRole(handler.RoleChief), settings.GetDemoMode)
			authGroup.PUT("/settings/demo-mode", handler.RequireRole(handler.RoleSuperAdmin), settings.SetDemoMode)

			// 修谱助手（AI）：仅理事长及以上（理事长/超级管理员）可用
			ai := &handler.AIHandler{DB: db}
			authGroup.GET("/ai/status", handler.RequireRole(handler.RoleChief), ai.GetStatus)
			authGroup.GET("/ai/config", handler.RequireRole(handler.RoleChief), ai.GetConfig)
			authGroup.PUT("/ai/config", handler.RequireRole(handler.RoleChief), ai.UpdateConfig)
			authGroup.POST("/ai/chat", handler.RequireRole(handler.RoleChief), ai.Chat)
			authGroup.POST("/ai/validate-save", handler.RequireRole(handler.RoleChief), ai.ValidateSave)
			authGroup.POST("/ai/polish-bio", handler.RequireRole(handler.RoleChief), ai.PolishBio)
			authGroup.POST("/ai/test", handler.RequireRole(handler.RoleChief), ai.Test)
			authGroup.POST("/ai/generate-bio", handler.RequireRole(handler.RoleChief), ai.GenerateBio)
			authGroup.POST("/ai/generate-biography", handler.RequireRole(handler.RoleChief), ai.GenerateBiography)

			// 在线更新包列表（浏览无需授权，下载安装需授权）
			update := &handler.UpdateHandler{DB: db, Cfg: cfg}
			authGroup.GET("/update/sources", handler.RequireRole(handler.RoleChief), update.ListUpdateSources)
			authGroup.GET("/update/server-packages", handler.RequireRole(handler.RoleChief), update.ListServerPackages)
			// 测试版（beta）更新：需测试版授权（handler 内门控），不需正式版授权
			authGroup.POST("/update/from-server", handler.RequireRole(handler.RoleChief), update.UpdateFromServer)
			// 更新进度查询（供 from-server / from-manifest 更新过程轮询）
			authGroup.GET("/update/progress", handler.RequireRole(handler.RoleChief), update.Progress)
			// 正式版更新：需正式版授权
			authGroup.GET("/update/check", handler.RequireRole(handler.RoleChief), update.CheckManifestUpdate)
			authGroup.POST("/update/from-manifest", handler.RequireRole(handler.RoleChief), update.UpdateFromManifest)
			authGroup.POST("/update/sources", handler.RequireRole(handler.RoleChief), update.SaveUpdateSources)
			authGroup.POST("/update/upload", handler.RequireRole(handler.RoleChief), update.UploadUpdate)
			authGroup.POST("/update/rollback", handler.RequireRole(handler.RoleChief), update.Rollback)

			// 操作日志（查看/清空：理事长及以上）
			authGroup.GET("/operation-logs", handler.RequirePriv("audit_log"), handler.ListOperationLogs(db))
			authGroup.DELETE("/operation-logs", handler.RequirePriv("audit_log"), handler.ClearOperationLogs(db))
			authGroup.GET("/settings/oplog-retention", handler.RequireRole(handler.RoleChief), handler.GetLogRetention(db))
			authGroup.PUT("/settings/oplog-retention", handler.RequireRole(handler.RoleChief), handler.UpdateLogRetention(db))

			// 收支公示（公开账目）：查看族员及以上；提交/审核/管理由 handler 内细粒度判定
			acc := &handler.AccountHandler{DB: db, Cfg: cfg}
			authGroup.GET("/accounts/config", handler.RequireRole(handler.RoleMember), acc.GetConfig)
			authGroup.PUT("/accounts/settings", handler.RequireRole(handler.RoleMember), acc.SaveSettings)
			authGroup.PUT("/accounts/fund", handler.RequireRole(handler.RoleMember), acc.FundFlow)
			authGroup.PUT("/accounts/submitters", handler.RequireRole(handler.RoleMember), acc.SetSubmitters)
			authGroup.GET("/accounts/books", handler.RequireRole(handler.RoleMember), acc.ListBooks)
			authGroup.POST("/accounts/books", handler.RequireRole(handler.RoleMember), acc.CreateBook)
			authGroup.GET("/accounts/books/:id", handler.RequireRole(handler.RoleMember), acc.GetBook)
			authGroup.PUT("/accounts/books/:id", handler.RequireRole(handler.RoleMember), acc.UpdateBook)
			authGroup.DELETE("/accounts/books/:id", handler.RequireRole(handler.RoleMember), acc.DeleteBook)
			authGroup.PUT("/accounts/books/:id/pin", handler.RequireRole(handler.RoleMember), acc.TogglePin)
			authGroup.PUT("/accounts/books/:id/merit", handler.RequireRole(handler.RoleMember), acc.SaveMeritConfig)
			authGroup.GET("/accounts/books/:id/entries", handler.RequireRole(handler.RoleMember), acc.ListEntries)
			authGroup.POST("/accounts/books/:id/entries", handler.RequireRole(handler.RoleMember), acc.CreateEntry)
			authGroup.GET("/accounts/pending", handler.RequireRole(handler.RoleMember), acc.ListPending)
			authGroup.PUT("/accounts/entries/:id", handler.RequireRole(handler.RoleMember), acc.UpdateEntry)
			authGroup.DELETE("/accounts/entries/:id", handler.RequireRole(handler.RoleMember), acc.DeleteEntry)
			authGroup.POST("/accounts/entries/:id/review", handler.RequireRole(handler.RoleMember), acc.ReviewEntry)
			authGroup.POST("/accounts/upload", handler.RequireRole(handler.RoleMember), acc.UploadVoucher)

			// 功德碑（可存在多个；新增/编辑走弹窗）
			authGroup.GET("/merit/steles", handler.RequireRole(handler.RoleMember), acc.ListMeritSteles)
			authGroup.POST("/merit/steles", handler.RequireRole(handler.RoleMember), acc.CreateMeritStele)
			authGroup.GET("/merit/steles/:id", handler.RequireRole(handler.RoleMember), acc.GetMeritStele)
			authGroup.PUT("/merit/steles/:id", handler.RequireRole(handler.RoleMember), acc.UpdateMeritStele)
			authGroup.DELETE("/merit/steles/:id", handler.RequireRole(handler.RoleMember), acc.DeleteMeritStele)
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
