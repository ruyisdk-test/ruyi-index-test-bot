package bot

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

func Serve(cfg *Config) {
	controlRouter := gin.Default()

	controlServer := http.Server{
		Addr:    cfg.ControlListenAddr,
		Handler: controlRouter,
	}

	router := newRouter()

	server := http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	err := db.Connect(cfg.ValkeyAddr)
	if err != nil {
		slog.Error("Valkey connect err:", "error", err.Error())
		return
	}

	schedule, err := CronStart(cfg)
	if err != nil {
		slog.Error("cron init failed:", "error", err.Error())
		return
	}

	// shutdown endpoint
	controlRouter.GET("/shutdown", func(c *gin.Context) {
		c.JSON(202, gin.H{
			"status": "shutting down",
		})

		go func() {
			CronStop(schedule)
			db.Close()
			err := server.Shutdown(context.Background())
			if err != nil {
				slog.Error("sent gin shutdown", "error", err.Error())
			}
			err = controlServer.Shutdown(context.Background())
			if err != nil {
				slog.Error("sent gin control shutdown", "error", err.Error())
			}
		}()
	})

	// cron
	controlRouter.GET("/cron", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"cron": GetCronEntryInfo(schedule),
		})
	})

	go func() {
		err = server.ListenAndServe()
		if err != nil {
			slog.Error("server listen and serve", "error", err.Error())
		}
	}()

	err = controlServer.ListenAndServe()
	if err != nil {
		slog.Error("control server listen and serve", "error", err.Error())
	}
}

func newRouter() *gin.Engine {
	router := gin.Default()

	router.GET("/version", version)

	newDbRouter(router)

	return router
}

func version(c *gin.Context) {
	if info, ok := debug.ReadBuildInfo(); ok {
		c.JSON(200, gin.H{
			"version": info.Main.Version,
		})
	} else {
		c.JSON(200, gin.H{
			"version": "unknown",
		})
	}
}

func newDbRouter(router *gin.Engine) {
	router.GET("/packages/search", pkgSearch)
	router.GET("/packages/types", pkgListGroups)
	router.GET("/packages/types/:pkg", pkgSearchGroupsByPkg)
	// ?page=&size=
	router.GET("/packages/packages", pkgListPackages)
	router.GET("/packages/packages/:type", pkgSearchPkgsByGroup)
	router.GET("/packages/versions/:type/:pkg", pkgListPackageVersions)
	router.GET("/packages/versions/:type/:pkg/:version", pkgSearchPackageVersion)
	router.GET("/packages/url", pkgSearchPkgsByUrl)
	router.GET("/packages/test/failure", testUrlFailure)
	router.GET("/packages/test/status", testUrlStatus)
}

func return500(c *gin.Context, err error) {
	slog.Error("list groups err:", "error", err.Error())
	c.JSON(500, gin.H{
		"status": "err",
		"msg":    err.Error(),
	})
}

func pkgSearch(c *gin.Context) {
	r, err := db.SearchPackages(c.Query("key"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgListGroups(c *gin.Context) {
	r, err := db.ListGroups(c.Request.Context())
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgListPackages(c *gin.Context) {
	ps := c.Query("page")
	// ss := c.Query("size")
	p, err := strconv.Atoi(ps)
	if err != nil {
		p = 0
	}
	// s, err := strconv.Atoi(ss)
	// if err != nil {
	// 	s = 50
	// }
	// fixed page size
	s := 50

	r, err := db.ListPackages(c.Request.Context(), p, s)
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgSearchGroupsByPkg(c *gin.Context) {
	r, err := db.GetGroupsByPkg(c.Request.Context(), c.Param("pkg"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgSearchPkgsByGroup(c *gin.Context) {
	r, err := db.GetPackagesByGroup(c.Request.Context(), c.Param("type"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgSearchPkgsByUrl(c *gin.Context) {
	r, err := db.GetPackagesByUrl(c.Request.Context(), c.Query("url"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgListPackageVersions(c *gin.Context) {
	r, err := db.GetPackageVersions(c.Request.Context(), c.Param("pkg"), c.Param("type"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func pkgSearchPackageVersion(c *gin.Context) {
	r, err := db.GetPackageVersionData(c.Request.Context(), c.Param("pkg"), c.Param("type"), c.Param("version"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func testUrlFailure(c *gin.Context) {
	r, err := db.ListUrlFailures(c.Request.Context())
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}

func testUrlStatus(c *gin.Context) {
	r, err := db.GetUrlTestStatus(c.Request.Context(), c.Query("url"))
	if err != nil {
		return500(c, err)
		return
	}

	c.JSON(200, r)
}
