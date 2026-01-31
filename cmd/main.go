package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/dovakiin0/proxy-m3u8/config"
	"github.com/dovakiin0/proxy-m3u8/internal/handler"
	mdlware "github.com/dovakiin0/proxy-m3u8/internal/middleware"
	"github.com/dovakiin0/proxy-m3u8/internal/utils"
)

func init() {
	godotenv.Load()
	config.InitConfig()
	config.RedisConnect()
}

func main() {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Pre(middleware.RemoveTrailingSlash())

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: utils.GetCorsDomain(),
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
	}))

	customCacheConfig := mdlware.CacheControlConfig{
		MaxAge:         3600, // 1 hour
		Public:         true,
		MustRevalidate: true,
	}
	e.Use(mdlware.CacheControlWithConfig(customCacheConfig))
	e.GET("/m3u8-proxy", handler.M3U8ProxyHandler)

	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "OK")
	})

	port := config.Env.Port

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
