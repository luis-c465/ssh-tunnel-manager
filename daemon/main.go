package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/tunnelmanager"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"google.golang.org/grpc"
)

const gracefulStopTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := tunnelmanager.NewTunnelManager()
	defer m.Shutdown()

	configDir := config.ConfigurationDir()
	if err := config.MigrateLegacyConfigDir(); err != nil {
		log.Printf("failed to migrate legacy configuration: %v", err)
		return
	}
	cf, err := configmanager.NewManagerWithError(configDir)
	if err != nil {
		log.Printf("failed to initialize configuration storage: %v", err)
		return
	}
	svc := tunnelmanager.NewTunnelService(m, cf, configDir)

	lis, err := net.Listen("tcp", config.Address)
	if err != nil {
		log.Printf("failed to listen on %s: %v", config.Address, err)
		return
	}
	defer lis.Close()

	s := grpc.NewServer()
	rpc.RegisterDaemonServiceServer(s, &server{service: svc, configManager: cf})

	if err := svc.RestoreTunnels(context.Background()); err != nil {
		log.Printf("failed to restore tunnels: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.Serve(lis)
	}()

	log.Printf("server listening at %v", lis.Addr())

	select {
	case <-ctx.Done():
		log.Printf("shutting down daemon: %v", ctx.Err())
	case err := <-serveErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("server stopped unexpectedly: %v", err)
		}
	}

	gracefulStop(s)
	if err := svc.PersistTunnels(); err != nil {
		log.Printf("failed to persist tunnels during shutdown: %v", err)
	}
}

func gracefulStop(s *grpc.Server) {
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(gracefulStopTimeout):
		log.Printf("graceful shutdown timed out; forcing server stop")
		s.Stop()
		<-done
	}
}
