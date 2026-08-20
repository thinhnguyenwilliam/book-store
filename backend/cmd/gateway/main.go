package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	_ "github.com/thinhnguyenwilliam/book-store/backend/docs"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	gatewayhttp "github.com/thinhnguyenwilliam/book-store/backend/internal/gateway/http"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/config"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcclient"
	appLogger "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// @title Book Store API
// @version 1.0
// @description Public HTTP API exposed by the Book Store API Gateway.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token using the format: Bearer {token}
func main() {
	os.Exit(execute())
}

func execute() int {
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load gateway config", "error", err)
		return 1
	}
	logManager, err := appLogger.New("gateway", cfg.Logging)
	if err != nil {
		slog.Error("initialize gateway logger", "error", err)
		return 1
	}
	slog.SetDefault(logManager.Logger())
	defer func() { _ = logManager.Close() }()

	if err := run(cfg); err != nil {
		slog.Error("gateway stopped", "error", err)
		return 1
	}
	return 0
}

func run(cfg config.Config) error {
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}
	requestTimeout, err := time.ParseDuration(cfg.Gateway.RequestTimeout)
	if err != nil {
		return err
	}
	performanceTarget, err := time.ParseDuration(cfg.Gateway.PerformanceTarget)
	if err != nil {
		return err
	}
	readHeaderTimeout, err := time.ParseDuration(cfg.Gateway.ReadHeaderTimeout)
	if err != nil {
		return err
	}
	readTimeout, err := time.ParseDuration(cfg.Gateway.ReadTimeout)
	if err != nil {
		return err
	}
	writeTimeout, err := time.ParseDuration(cfg.Gateway.WriteTimeout)
	if err != nil {
		return err
	}
	idleTimeout, err := time.ParseDuration(cfg.Gateway.IdleTimeout)
	if err != nil {
		return err
	}

	authConnection, err := grpc.NewClient(
		cfg.GRPC.AuthAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryLoggingInterceptor),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer func() { _ = authConnection.Close() }()

	userConnection, err := grpc.NewClient(
		cfg.GRPC.UserAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryLoggingInterceptor),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer func() { _ = userConnection.Close() }()

	bookConnection, err := grpc.NewClient(
		cfg.GRPC.BookAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcclient.UnaryLoggingInterceptor),
		grpc.WithChainStreamInterceptor(grpcclient.StreamLoggingInterceptor),
	)
	if err != nil {
		return err
	}
	defer func() { _ = bookConnection.Close() }()

	handler := gatewayhttp.NewHandler(
		bookstorev1.NewAuthServiceClient(authConnection),
		bookstorev1.NewUserServiceClient(userConnection),
		bookstorev1.NewBookServiceClient(bookConnection),
		gatewayhttp.RefreshCookieConfig{
			Name:     cfg.Gateway.RefreshCookieName,
			Secure:   cfg.Gateway.RefreshCookieSecure,
			SameSite: sameSiteMode(cfg.Gateway.RefreshCookieSameSite),
		},
		cfg.Gateway.AllowedOrigins,
	)

	e := echo.New()
	e.HideBanner = true
	e.Server.ReadHeaderTimeout = readHeaderTimeout
	e.Server.ReadTimeout = readTimeout
	e.Server.WriteTimeout = writeTimeout
	e.Server.IdleTimeout = idleTimeout
	e.Use(middleware.RequestID())
	e.Use(gatewayhttp.TraceID)
	e.Use(gatewayhttp.RequestDeadline(requestTimeout))
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogRequestID:    true,
		LogRemoteIP:     true,
		LogMethod:       true,
		LogURI:          true,
		LogStatus:       true,
		LogLatency:      true,
		LogResponseSize: true,
		LogError:        true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			sloMet := values.Latency < performanceTarget
			attributes := []any{
				"request_id", values.RequestID,
				"remote_ip", values.RemoteIP,
				"method", values.Method,
				"uri", values.URI,
				"status", values.Status,
				"duration_ms", float64(values.Latency.Microseconds()) / 1000,
				"response_bytes", values.ResponseSize,
				"slo_target_ms", float64(performanceTarget.Microseconds()) / 1000,
				"slo_met", sloMet,
			}
			if values.Error != nil || !sloMet {
				attributes = append(attributes, "error", values.Error)
				slog.WarnContext(c.Request().Context(), "HTTP request completed", attributes...)
			} else {
				slog.InfoContext(c.Request().Context(), "HTTP request completed", attributes...)
			}
			return nil
		},
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.Gateway.AllowedOrigins,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Trace-ID"},
		ExposeHeaders:    []string{echo.HeaderXRequestID, "X-Trace-ID"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	e.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	handler.RegisterRoutes(e)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		addr := cfg.Gateway.HTTPAddress
		slog.Info("HTTP gateway started", "address", addr)
		errCh <- e.Start(addr)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil //nolint:nilerr // Echo returns this sentinel after a normal shutdown.
		}
		return err
	case <-ctx.Done():
		slog.Info("HTTP gateway graceful shutdown started", "timeout", shutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			return err
		}
		slog.Info("HTTP gateway graceful shutdown completed")
		return nil
	}
}

func sameSiteMode(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
