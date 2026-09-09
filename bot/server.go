package bot

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func Serve(cfg *Config) {
	router := newRouter()

	server := http.Server{
		Addr:    cfg.ListenAddr,
		Handler: router,
	}

	schedule, err := CronStart(cfg)
	if err != nil {
		slog.Error("cron init failed:", "error", err.Error())
		return
	}

	// shutdown endpoint
	router.GET("/shutdown", func(c *gin.Context) {
		c.JSON(202, gin.H{
			"status": "shutting down",
		})

		go func() {
			CronStop(schedule)
			err := server.Shutdown(context.Background())
			if err != nil {
				slog.Error("sent gin shutdown", "error", err.Error())
			}
		}()
	})

	// cron
	router.GET("/cron", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"cron": GetCronEntryInfo(schedule),
		})
	})

	err = server.ListenAndServe()
	if err != nil {
		slog.Error("gin listen and serve", "error", err.Error())
	}
}

func newRouter() *gin.Engine {
	router := gin.Default()

	router.GET("/version", version)

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
