package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	pb "github.com/besrabasant/ssh-tunnel-manager/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const daemonDialTimeout = 5 * time.Second

func CreateDaemonServiceClient() (pb.DaemonServiceClient, func(), error) {
	conn, err := grpc.NewClient(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("did not connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), daemonDialTimeout)
	defer cancel()

	conn.Connect()
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		if !conn.WaitForStateChange(ctx, state) {
			conn.Close()
			return nil, nil, fmt.Errorf("did not connect: %w", ctx.Err())
		}
	}

	cleanup := func() { conn.Close() }
	c := pb.NewDaemonServiceClient(conn)
	return c, cleanup, nil
}
