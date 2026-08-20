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
	configPath := flag.String("config", "config/config.yml", "path to YAML configuration")
	flag.Parse()
	if err := run(*configPath); err != nil {
		slog.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil {
		return err
	}

	authConnection, err := grpc.NewClient(
		cfg.GRPC.AuthAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer authConnection.Close()

	userConnection, err := grpc.NewClient(
		cfg.GRPC.UserAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer userConnection.Close()

	bookConnection, err := grpc.NewClient(
		cfg.GRPC.BookAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer bookConnection.Close()

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
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.Gateway.AllowedOrigins,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
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
			return nil
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
