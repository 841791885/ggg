package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"ggg/config"
	"ggg/controllers"
	"ggg/database"
	"ggg/repositories"
	"ggg/routes"
	"ggg/services"
)

func main() {
	// 所有运行参数都从配置文件读取，避免把端口、账号等环境信息散落在代码中。
	appConfig, err := config.Load(config.Path())
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// 数据库连接不能无限等待。这里使用配置中的超时时间限制首次连接过程。
	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		appConfig.Database.ConnectTimeout.Value(),
	)
	gormDB, sqlDB, err := database.OpenMySQL(databaseContext, appConfig.Database)
	cancelDatabase()
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	defer sqlDB.Close()

	healthController := controllers.NewHealthController(sqlDB, 2*time.Second)

	// main 是程序的组装入口：Repository -> Service -> Controller -> Router。
	productRepository := repositories.NewMySQLRepository(gormDB)
	productService := services.NewProductService(productRepository)
	productController := controllers.NewProductController(productService)

	router, err := routes.New(healthController, productController)
	if err != nil {
		log.Fatalf("创建路由失败: %v", err)
	}

	server := &http.Server{
		Addr:              appConfig.Server.Address,
		Handler:           router,
		ReadHeaderTimeout: appConfig.Server.ReadHeaderTimeout.Value(),
	}

	// ListenAndServe 会阻塞当前 goroutine，因此放到后台运行，并通过 channel
	// 把启动失败、端口占用等错误传回主 goroutine。
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	// SIGINT 对应 Ctrl+C，SIGTERM 通常由 Docker、进程管理器等发出。
	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
		return
	case <-shutdownSignal.Done():
	}

	// Shutdown 会停止接收新请求，并在超时前等待正在处理的请求结束。
	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		appConfig.Server.ShutdownTimeout.Value(),
	)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("HTTP 服务关闭失败: %v", err)
	}

	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP 服务退出错误: %v", err)
	}
}
