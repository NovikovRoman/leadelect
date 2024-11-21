package node

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	pb "github.com/NovikovRoman/leadelect/grpc"
)

type initNodeResult struct {
	nodeID string
	resp   *pb.NodeStatusResponse
	err    error
}

func (n *Node) Run(ctx context.Context) {
	go n.serverRun(ctx)

	// And who is the leader here?
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
			n.logger.Info(ctx, "Run", map[string]any{
				"status": n.getStatus(),
			})
			time.Sleep(time.Second)

		} else {
			nextCheck = time.Now().Add(n.electionCheckTimeout)
		}
	}
}

func (n *Node) serverRun(ctx context.Context) {
	lis, err := net.Listen("tcp", n.AddrPort())
	if err != nil {
		n.logger.Err(ctx, fmt.Errorf("serverRun net.Listen: %w", err), map[string]any{
			"id":   n.ID(),
			"addr": n.AddrPort(),
		})
		return
	}

	pb.RegisterNodeServer(n.grpcServer, newServer(n))

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		n.grpcServer.GracefulStop()
		n.logger.Info(ctx, "serverRun Graceful stop", map[string]any{
			"id":   n.ID(),
			"addr": n.AddrPort(),
		})
		wg.Done()
	}()

	n.logger.Info(ctx, "serverRun", map[string]any{
		"id":   n.ID(),
		"addr": n.AddrPort(),
	})
	if err := n.grpcServer.Serve(lis); err != nil {
		n.logger.Err(ctx, fmt.Errorf("serverRun grpcServer.Serve: %w", err), map[string]any{
			"id":   n.ID(),
			"addr": n.AddrPort(),
		})
	}
	wg.Wait()
}

func (n *Node) initNodes(ctx context.Context) {
	ch := make(chan initNodeResult)
	defer close(ch)

	go func() {
		wg := &sync.WaitGroup{}
		for _, node := range n.Nodes() {
			wg.Add(1)
			go n.initNode(ctx, wg, ch, node)
		}
		wg.Wait()
	}()

	maxRound := int64(0)
	hasLeader := false
	for i := 0; i < len(n.Nodes()); i++ {
		select {
		case <-ctx.Done():
			return

		case r := <-ch:
			if r.err != nil {
				n.logger.Info(ctx, "initNode grpcClient.status", map[string]any{
					"nodeID": r.nodeID,
					"err":    r.err,
				})
				continue
			}

			n.logger.Info(ctx, "initNode response.status", map[string]any{
				"nodeID": r.nodeID,
				"status": r.resp.Status,
				"round":  r.resp.Round,
			})

			if r.resp.Status == pb.NodeStatus_Leader {
				hasLeader = true
				n.newLeader(r.nodeID)
				n.setHeartbeat()
				n.setRound(r.resp.Round)

			} else if r.resp.Round > maxRound {
				maxRound = r.resp.Round
			}
		}
	}

	if !hasLeader && maxRound > n.getRound() {
		n.setRound(maxRound)
	}
}

func (n *Node) initNode(ctx context.Context, wg *sync.WaitGroup, ch chan initNodeResult, node *Node) {
	defer wg.Done()

	res := initNodeResult{
		nodeID: node.ID(),
		resp:   nil,
	}

	resp, err := n.grpcClient.status(ctx, node.AddrPort())
	if err != nil {
		res.err = err
		ch <- res
		return
	}

	ch <- initNodeResult{
		nodeID: node.ID(),
		resp:   resp,
	}
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

	ch := make(chan string)
	defer close(ch)

	wg := &sync.WaitGroup{}

	maxRound := int64(0)
	for _, node := range n.Nodes() {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			if n.isFollower() {
				return
			}

			resp, err := n.grpcClient.vote(ctx, node.AddrPort())
			if err != nil {
				n.logger.Err(ctx, fmt.Errorf("election vote: %w", err), map[string]any{
					"nodeID": node.id,
				})
				return
			}
			n.logger.Info(ctx, "election", map[string]any{
				"nodeID": node.id,
				"vote":   resp.Vote,
				"round":  resp.Round,
			})
			if maxRound < resp.Round {
				maxRound = resp.Round
			}
			if resp.Vote {
				n.addVote()
			}
		}(wg)
	}
	wg.Wait()

	n.logger.Info(ctx, "election", map[string]any{
		"votes": n.numVotes(),
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
