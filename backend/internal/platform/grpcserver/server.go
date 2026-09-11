package grpcserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func Run(ctx context.Context, addr string, shutdownTimeout time.Duration, register func(*grpc.Server), interceptors ...grpc.UnaryServerInterceptor) error {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return run(ctx, listener, shutdownTimeout, register, interceptors...)
}

func run(ctx context.Context, listener net.Listener, shutdownTimeout time.Duration, register func(*grpc.Server), interceptors ...grpc.UnaryServerInterceptor) error {
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(append([]grpc.UnaryServerInterceptor{unaryInterceptor}, interceptors...)...),
		grpc.ChainStreamInterceptor(streamInterceptor),
	)
	register(server)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("gRPC server started", "address", listener.Addr().String())
		errCh <- server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("gRPC graceful shutdown started", "timeout", shutdownTimeout)
		healthServer.Shutdown()
		stopped := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			slog.Info("gRPC graceful shutdown completed")
			return nil
		case <-time.After(shutdownTimeout):
			slog.Warn("gRPC graceful shutdown timed out; forcing stop", "timeout", shutdownTimeout)
			server.Stop()
			return errors.New("gRPC server forced to stop after timeout")
		}
	}
}
