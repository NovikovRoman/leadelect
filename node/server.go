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

func (s *server) Status(ctx context.Context, in *pb.EmptyRequest) (*pb.NodeStatusResponse, error) {
	return &pb.NodeStatusResponse{
		Status: s.node.Status(),
		Round:  s.node.getRound(),
	}, nil
}

func (s *server) Vote(ctx context.Context, in *pb.VoteRequest) (*pb.VoteResponse, error) {
	return &pb.VoteResponse{
		Vote:  s.node.agreeVote(in.Round),
		Round: s.node.getRound(),
	}, nil
}

func (s *server) Heartbeat(ctx context.Context, in *pb.HeartbeatRequest) (*pb.EmptyResponse, error) {
	s.node.setHeartbeat()

	leader := s.node.getLeader()
	if leader == nil || leader.getRound() < in.Round {
		s.node.logger.Info(ctx, "Heartbeat", []LoggerField{
			{Key: "Leader", Value: in.NodeID},
		})
		s.node.newLeader(in.NodeID)
		s.node.setRound(in.Round)
	}
	return &pb.EmptyResponse{}, nil
}
