package node

import (
	"context"

	pb "github.com/NovikovRoman/leadelect/grpc"
	"google.golang.org/grpc"
)

type client struct {
	node           *Node
	grpcClientOpts []grpc.DialOption
}

func newClient(node *Node, opts []grpc.DialOption) (c *client) {
	return &client{
		node:           node,
		grpcClientOpts: opts,
	}
}

func (c *client) heartbeat(ctx context.Context, target string) (err error) {
	ctx, cancel := context.WithTimeout(ctx, c.node.clientTimeout)
	defer cancel()

	conn, err := grpc.NewClient(target, c.grpcClientOpts...)
	if err != nil {
		return
	}
	defer conn.Close()

	client := pb.NewNodeClient(conn)
	_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{NodeID: c.node.id, Round: c.node.getRound()})
	return
}

func (c *client) vote(ctx context.Context, target string) (*pb.VoteResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.node.clientTimeout)
	defer cancel()

	conn, err := grpc.NewClient(target, c.grpcClientOpts...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewNodeClient(conn)
	return client.Vote(ctx, &pb.VoteRequest{NodeID: c.node.id, Round: c.node.getRound()})
}

func (c *client) status(ctx context.Context, target string) (status *pb.NodeStatusResponse, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.node.clientTimeout)
	defer cancel()

	conn, err := grpc.NewClient(target, c.grpcClientOpts...)
	if err != nil {
		return
	}
	defer conn.Close()

	client := pb.NewNodeClient(conn)
	return client.Status(ctx, &pb.EmptyRequest{})
}
