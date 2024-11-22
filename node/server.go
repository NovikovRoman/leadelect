package node

import (
	"context"

	pb "github.com/NovikovRoman/leadelect/grpc"
)

type server struct {
	pb.UnimplementedNodeServer
	node *Node
}

func newServer(node *Node) *server {
	return &server{
		node: node,
	}
}

// Status returns the status and voting round number of the node.
func (s *server) Status(ctx context.Context, in *pb.EmptyRequest) (*pb.NodeStatusResponse, error) {
	return &pb.NodeStatusResponse{
		Status: s.node.Status(),
		Round:  s.node.getRound(),
	}, nil
}

// Vote handles a voting request from another node.
func (s *server) Vote(ctx context.Context, in *pb.VoteRequest) (*pb.VoteResponse, error) {
	return &pb.VoteResponse{
		Vote:  s.node.agreeVote(in.Round),
		Round: s.node.getRound(),
	}, nil
}

// Heartbeat processes a heartbeat from the leader.
func (s *server) Heartbeat(ctx context.Context, in *pb.HeartbeatRequest) (*pb.EmptyResponse, error) {
	s.node.setHeartbeat()

	leader := s.node.getLeader()
	if leader == nil || leader.getRound() < in.Round {
		s.node.logger.Info(ctx, "Heartbeat", []LoggerField{
			{Key: "New leader", Value: in.NodeID},
		})
		s.node.newLeader(in.NodeID)
		s.node.setRound(in.Round)
	}
	return &pb.EmptyResponse{}, nil
}
