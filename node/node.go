package node

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/NovikovRoman/leadelect/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Node struct {
	sync.RWMutex
	id     string
	addr   string
	port   int
	status pb.NodeStatus

	round int64 // Voting round
	votes int   // Votes from the nodes
	voted bool

	nodes map[string]*Node // Other nods

	// For the leader it is the time of the last heartbeat.
	// For followers, this is the time of the last reply on heartbeat.
	heartbeatTime           time.Time
	heartbeatTimeout        time.Duration // Maximum wait for a response to heartbeat.
	heartbeatExpiredTimeout time.Duration // Maximum waiting time heartbeat.

	electionCheckTimeout time.Duration

	grpcClient    *client
	clientTimeout time.Duration
	grpcServer    *grpc.Server

	logger Logger
}

type NodeOpt func(n *Node)

func ClientTimeout(timeout time.Duration) NodeOpt {
	return func(n *Node) {
		n.clientTimeout = timeout
	}
}

func HeartbeatExpiredTimeout(timeout time.Duration) NodeOpt {
	return func(n *Node) {
		n.heartbeatExpiredTimeout = timeout
	}
}

func HeartbeatTimeout(timeout time.Duration) NodeOpt {
	return func(n *Node) {
		n.heartbeatTimeout = timeout
	}
}

func CheckElectionTimeout(timeout time.Duration) NodeOpt {
	return func(n *Node) {
		n.electionCheckTimeout = timeout
	}
}

func WithLogger(logger Logger) NodeOpt {
	return func(n *Node) {
		n.logger = logger
	}
}

func New(id, addr string, port int, opts ...NodeOpt) (n *Node) {
	n = &Node{
		id:                      id,
		addr:                    addr,
		port:                    port,
		nodes:                   make(map[string]*Node),
		status:                  pb.NodeStatus_Follower,
		heartbeatExpiredTimeout: time.Second * 5,
		clientTimeout:           time.Second * 10,
		heartbeatTimeout:        time.Second * 3,
		electionCheckTimeout:    time.Second * 10,
		grpcServer:              grpc.NewServer(),
		logger:                  NewLogger(slog.LevelDebug),
	}

	for _, opt := range opts {
		opt(n)
	}

	n.grpcClient = newClient(n, []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	})
	return
}

func (n *Node) ClientTLS(caFile, serverHostOverride string) (err error) {
	caFile = absPath(caFile)
	var creds credentials.TransportCredentials
	if creds, err = credentials.NewClientTLSFromFile(caFile, serverHostOverride); err != nil {
		return
	}
	n.grpcClient = newClient(n, []grpc.DialOption{grpc.WithTransportCredentials(creds)})
	return
}

func (n *Node) ServerTLS(certFile, keyFile string) (err error) {
	var creds credentials.TransportCredentials
	creds, err = credentials.NewServerTLSFromFile(absPath(certFile), absPath(keyFile))
	if err != nil {
		return
	}
	n.grpcServer = grpc.NewServer(grpc.Creds(creds))
	return
}

func (n *Node) ID() string {
	return n.id
}

func (n *Node) Addr() string {
	return n.addr
}

func (n *Node) Port() int {
	return n.port
}

func (n *Node) AddrPort() string {
	return fmt.Sprintf("%s:%d", n.addr, n.port)
}

func (n *Node) AddNode(node ...*Node) {
	n.Lock()
	for _, nn := range node {
		n.nodes[nn.id] = nn
	}
	n.Unlock()
}

func (n *Node) Nodes() map[string]*Node {
	n.RLock()
	defer n.RUnlock()
	return n.nodes
}

func (n *Node) NumNodes() int {
	n.RLock()
	defer n.RUnlock()
	return len(n.nodes)
}

func (n *Node) Status() pb.NodeStatus {
	n.RLock()
	defer n.RUnlock()
	return n.status
}

func (n *Node) addVote() {
	n.Lock()
	n.votes += 1
	n.Unlock()
}

func (n *Node) numVotes() int {
	n.RLock()
	defer n.RUnlock()
	return n.votes
}

func (n *Node) resetVotes() {
	n.Lock()
	n.votes = 0
	n.Unlock()
}

func (n *Node) isVoted() bool {
	n.RLock()
	defer n.RUnlock()
	return n.voted
}

func (n *Node) setVoted() {
	n.Lock()
	n.voted = true
	n.Unlock()
}

func (n *Node) resetVoted() {
	n.Lock()
	n.voted = false
	n.Unlock()
}

func (n *Node) getRound() int64 {
	n.RLock()
	defer n.RUnlock()
	return n.round
}

func (n *Node) setRound(round int64) {
	n.Lock()
	n.round = round
	n.Unlock()
}

func (n *Node) addRound() {
	n.Lock()
	n.round += 1
	n.Unlock()
}

func (n *Node) agreeVote(round int64) bool {
	n.Lock()
	defer n.Unlock()

	if n.status != pb.NodeStatus_Follower && n.round >= round || n.voted {
		return false
	}

	n.voted = true
	n.votes = 0
	n.status = pb.NodeStatus_Follower // withdraw
	return true
}

func (n *Node) heartbeatExpired() bool {
	n.RLock()
	defer n.RUnlock()
	return time.Since(n.heartbeatTime) > n.heartbeatExpiredTimeout
}

func (n *Node) setHeartbeat() {
	n.Lock()
	n.heartbeatTime = time.Now()
	n.Unlock()
}

func (n *Node) setStatus(status pb.NodeStatus) {
	n.Lock()
	n.status = status
	n.Unlock()
}

func (n *Node) getStatus() pb.NodeStatus {
	n.RLock()
	defer n.RUnlock()
	return n.status
}

func (n *Node) isLeader() bool {
	return n.getStatus() == pb.NodeStatus_Leader
}

func (n *Node) isFollower() bool {
	return n.getStatus() == pb.NodeStatus_Follower
}

func (n *Node) newLeader(nodeID string) {
	n.Lock()
	defer n.Unlock()

	n.voted = false
	n.votes = 0

	if n.id == nodeID {
		n.status = pb.NodeStatus_Leader
	} else {
		n.status = pb.NodeStatus_Follower
	}

	for id, node := range n.nodes {
		if id == nodeID {
			node.status = pb.NodeStatus_Leader
		} else {
			node.status = pb.NodeStatus_Follower
		}
	}
}

func (n *Node) getLeader() *Node {
	n.RLock()
	defer n.RUnlock()

	if n.status == pb.NodeStatus_Leader {
		return n
	}

	for _, node := range n.nodes {
		if node.status == pb.NodeStatus_Leader {
			return node
		}
	}
	return nil
}

func (n *Node) sendHeartbeat(ctx context.Context) error {
	n.setHeartbeat()

	var err error
	numErrors := 0

	chErr := make(chan error)
	go func() {
		for e := range chErr {
			if e != nil {
				err = errors.Join(err, e)
				numErrors++
			}
		}
	}()

	wg := &sync.WaitGroup{}
	for _, node := range n.Nodes() {
		wg.Add(1)
		go n.sendHeartbeatWorker(ctx, wg, chErr, node)
	}
	wg.Wait()
	close(chErr)

	if numErrors > n.NumNodes()/2 {
		n.setStatus(pb.NodeStatus_Follower) // withdraw
		n.logger.Warn(ctx, "heartbeat", map[string]any{
			"numNonReplies": numErrors,
			"numNodes":      n.NumNodes(),
			"status":        n.getStatus(),
		})
	}
	return err
}

func (n *Node) sendHeartbeatWorker(ctx context.Context, wg *sync.WaitGroup, chErr chan<- error, node *Node) {
	defer wg.Done()
	if err := n.grpcClient.heartbeat(ctx, node.AddrPort()); err != nil {
		chErr <- fmt.Errorf("node %s: %w", node.ID(), err)
		return
	}

	node.setHeartbeat()
	chErr <- nil
}
