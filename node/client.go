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

func newClient(node *Node, opts []grpc.DialOption) *client {
	return &client{
		node:           node,
		grpcClientOpts: opts,
	}
}

// heartBeat sends a heartbeat message to the target node and returns an error if unsuccessful.
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

// vote sends a vote request to the target node and returns the response or an error if unsuccessful.
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

// status retrieves the status from the target node and returns the response or an error if unsuccessful.
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
