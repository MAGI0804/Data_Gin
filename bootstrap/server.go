package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/logger"
)

// RunServer 启动服务
func RunServer() {

	console.Info("run server ...")

	// 设置 gin 框架的运行模式，支持 debug, release, test
	// release 会屏蔽调试信息，官方建议生产环境中使用
	// 非 release 模式 gin 终端会打印调试信息
	gin.SetMode(config.GetString("cfg.app.gin_run_mode"))
	// gin 实例
	router := gin.New()
	// 初始化路由绑定
	setupRouter(router)
	// 运行服务器
	srv := initServer(router)

	// 检查是否启用 SSL
	if config.GetBool("cfg.app.ssl.enabled") {
		certFile := config.GetString("cfg.app.ssl.cert_file")
		keyFile := config.GetString("cfg.app.ssl.key_file")
		domain := config.GetString("cfg.app.ssl.domain")
		sslPort := config.GetString("cfg.app.ssl.port")

		console.Success("HTTPS Server is running at: https://%s:%s", domain, sslPort)
		// 优雅的重启和停止 (HTTPS)
		gracefulShutdownHTTPS(srv, certFile, keyFile)
	} else {
		console.Success("HTTP Server is running at: http://0.0.0.0:%d", config.GetInt("cfg.app.port"))
		// 优雅的重启和停止 (HTTP)
		gracefulShutdown(srv)
	}
}

// gracefulShutdown 优雅的重启和停止 (HTTP)
func gracefulShutdown(srv *http.Server) {
	// 优雅的重启和停止
	// see gin web framework document examples : https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/notify-without-context/server.go
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorString("Server", "gracefulShutdown", err.Error())
			console.Exit("server.ListenAndServe err: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal)
	// 接受 syscall.SIGINT 和 syscall.SIGTERM 信号
	// kill 不加参数发送 syscall.SIGTERM 信号
	// kill -2 发送 syscall.SIGINT 信号
	// kill -9 发送 syscall.SIGKILL 信号，但是不能被捕获，因此不需要添加它
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	console.Warning("Shutting down server...")
	logger.WarnString("Server", "gracefulShutdown", "正在关闭服务器……")

	// 最大时间控制，用于通知该服务端它有 5 秒的时间来处理原有的请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.FatalString("Server", "gracefulShutdown", err.Error())
	}

	console.Warning("Server exiting")
	logger.WarnString("Server", "gracefulShutdown", "服务已经退出")

}

// gracefulShutdownHTTPS 优雅的重启和停止 (HTTPS)
func gracefulShutdownHTTPS(srv *http.Server, certFile, keyFile string) {
	// 优雅的重启和停止
	// see gin web framework document examples : https://github.com/gin-gonic/examples/blob/master/graceful-shutdown/graceful-shutdown/notify-without-context/server.go
	go func() {
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			logger.ErrorString("Server", "gracefulShutdownHTTPS", err.Error())
			console.Exit("server.ListenAndServeTLS err: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal)
	// 接受 syscall.SIGINT 和 syscall.SIGTERM 信号
	// kill 不加参数发送 syscall.SIGTERM 信号
	// kill -2 发送 syscall.SIGINT 信号
	// kill -9 发送 syscall.SIGKILL 信号，但是不能被捕获，因此不需要添加它
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	console.Warning("Shutting down server...")
	logger.WarnString("Server", "gracefulShutdownHTTPS", "正在关闭服务器……")

	// 最大时间控制，用于通知该服务端它有 5 秒的时间来处理原有的请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.FatalString("Server", "gracefulShutdownHTTPS", err.Error())
	}

	console.Warning("Server exiting")
	logger.WarnString("Server", "gracefulShutdownHTTPS", "服务已经退出")

}

// initServer 初始化服务器
func initServer(router *gin.Engine) *http.Server {
	var addr string

	// 检查是否启用 SSL
	if config.GetBool("cfg.app.ssl.enabled") {
		// 使用 SSL 域名和端口
		domain := config.GetString("cfg.app.ssl.domain")
		sslPort := config.GetString("cfg.app.ssl.port")
		addr = fmt.Sprintf("%s:%s", domain, sslPort)
	} else {
		// 使用默认端口
		addr = fmt.Sprintf(":%d", config.GetInt("cfg.app.port"))
	}

	return &http.Server{
		Addr:           addr, // 服务启动的地址和端口
		Handler:        router,
		ReadTimeout:    time.Second * time.Duration(config.GetInt64("cfg.app.read_timeout")),  // 允许读取的最大时间
		WriteTimeout:   time.Second * time.Duration(config.GetInt64("cfg.app.write_timeout")), // 允许写入的最大时间
		MaxHeaderBytes: 1 << 20,                                                               // 请求头的最大字节数
	}
}
