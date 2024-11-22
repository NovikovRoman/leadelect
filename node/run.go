package node

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/NovikovRoman/leadelect/grpc"
)

type initNodeResult struct {
	nodeID string
	resp   *pb.NodeStatusResponse
	err    error
}

func (n *Node) Run(ctx context.Context) {
	go func() {
		if err := n.serverRun(ctx); err != nil {
			n.logger.Err(ctx, fmt.Errorf("Run serverRun: %w", err), []LoggerField{
				{Key: "id", Value: n.ID()},
				{Key: "addr", Value: n.AddrPort()},
			})
		}
	}()

	// initializing nodes
	n.initNodes(ctx)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	nextCheck := time.Now()
	nextHeartbeat := time.Now()
	for t := range ticker.C {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if n.isLeader() && t.After(nextHeartbeat) {
			n.setHeartbeat()
			nextHeartbeat = time.Now().Add(n.heartbeatTimeout)
			if err := n.sendHeartbeat(ctx); err != nil {
				n.logger.Err(ctx, fmt.Errorf("Run heartbeat: %w", err), nil)
			}

		} else if n.getLeader() != nil && n.heartbeatExpired() {
			n.newLeader("")
		}

		if n.getLeader() != nil && t.Before(nextCheck) {
			continue
		}

		if n.getLeader() == nil {
			n.election(ctx)
			n.logger.Info(ctx, "Run", []LoggerField{
				{Key: "status", Value: n.getStatus()},
			})
			time.Sleep(time.Second)

		} else {
			nextCheck = time.Now().Add(n.electionCheckTimeout)
		}
	}
}

func (n *Node) serverRun(ctx context.Context) (err error) {
	lis, err := net.Listen("tcp", n.AddrPort())
	if err != nil {
		err = fmt.Errorf("serverRun net.Listen: %w", err)
		return
	}

	pb.RegisterNodeServer(n.grpcServer, newServer(n))

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		n.grpcServer.GracefulStop()
		n.logger.Info(ctx, "serverRun Graceful stop", []LoggerField{
			{Key: "id", Value: n.ID()},
			{Key: "addr", Value: n.AddrPort()},
		})
		wg.Done()
	}()

	n.logger.Info(ctx, "serverRun", []LoggerField{
		{Key: "id", Value: n.ID()},
		{Key: "addr", Value: n.AddrPort()},
	})
	if err = n.grpcServer.Serve(lis); err != nil {
		err = fmt.Errorf("serverRun grpcServer.Serve: %w", err)
	}
	wg.Wait()
	return
}

func (n *Node) initNodes(ctx context.Context) {
	ch := make(chan initNodeResult)
	defer close(ch)

	go func() {
		wg := &sync.WaitGroup{}
		for _, node := range n.Nodes() {
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()
				ch <- n.initNode(ctx, node)
			}(wg)
		}
		wg.Wait()
	}()

	maxRound := int64(0)
	findLeader := false
	for i := 0; i < len(n.Nodes()); i++ {
		select {
		case <-ctx.Done():
			return

		case r := <-ch:
			if r.err != nil {
				n.logger.Info(ctx, "initNode grpcClient.status", []LoggerField{
					{Key: "nodeID", Value: r.nodeID},
					{Key: "err", Value: r.err},
				})
				continue
			}

			n.logger.Info(ctx, "initNode response.status", []LoggerField{
				{Key: "nodeID", Value: r.nodeID},
				{Key: "status", Value: r.resp.Status},
				{Key: "round", Value: r.resp.Round},
			})

			if r.resp.Status == pb.NodeStatus_Leader {
				findLeader = true
				n.newLeader(r.nodeID)
				n.setHeartbeat()
				n.setRound(r.resp.Round)

			} else if r.resp.Round > maxRound {
				maxRound = r.resp.Round
			}
		}
	}

	if !findLeader && maxRound > n.getRound() {
		n.setRound(maxRound)
	}
}

func (n *Node) initNode(ctx context.Context, node *Node) (r initNodeResult) {
	r = initNodeResult{
		nodeID: node.ID(),
		resp:   nil,
	}

	resp, err := n.grpcClient.status(ctx, node.AddrPort())
	if err != nil {
		r.err = err
		return
	}

	r.resp = resp
	return
}

func (n *Node) election(ctx context.Context) {
	n.resetVotes()
	if n.isVoted() {
		n.setStatus(pb.NodeStatus_Follower)
		return
	}

	time.Sleep(time.Millisecond * time.Duration(rand.Int32N(150)+150))

	n.addRound()
	n.setVoted()
	n.addVote()
	n.setStatus(pb.NodeStatus_Candidate)

	maxRound := n.votingNodes(ctx)

	n.logger.Info(ctx, "election", []LoggerField{
		{Key: "votes", Value: n.numVotes()},
		{Key: "round", Value: maxRound},
	})

	if n.numVotes() > 0 && n.numVotes() > n.NumNodes()/2 {
		n.newLeader(n.id)
		if err := n.sendHeartbeat(ctx); err != nil {
			n.logger.Err(ctx, fmt.Errorf("election sendHeartbeat: %w", err), nil)
		}
		return

	} else if maxRound > n.getRound() {
		n.setRound(maxRound)
	}

	n.resetVotes()
	n.setStatus(pb.NodeStatus_Follower)
	n.resetVoted()
}

func (n *Node) votingNodes(ctx context.Context) int64 {
	if n.isFollower() || n.isLeader() {
		return n.getRound()
	}

	var aMaxRound atomic.Int64
	wg := &sync.WaitGroup{}
	for _, node := range n.Nodes() {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			resp, err := n.grpcClient.vote(ctx, node.AddrPort())
			if err != nil {
				n.logger.Err(ctx, fmt.Errorf("election vote: %w", err), []LoggerField{
					{Key: "nodeID", Value: node.id},
				})
				return
			}

			n.logger.Info(ctx, "election", []LoggerField{
				{Key: "nodeID", Value: node.id},
				{Key: "vote", Value: resp.Vote},
				{Key: "round", Value: resp.Round},
			})

			if aMaxRound.Load() < resp.Round {
				aMaxRound.Store(resp.Round)
			}
			if resp.Vote {
				n.addVote()
			}
		}(wg)
	}
	wg.Wait()
	return aMaxRound.Load()
}
