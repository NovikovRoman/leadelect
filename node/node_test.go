//go:build !race

package node

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	pb "github.com/NovikovRoman/leadelect/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

var (
	testNodeHeartbeatPort = 1111
)

func TestNodeOpts(t *testing.T) {
	opts := []NodeOpt{
		ClientTimeout(time.Second * 11),
		HeartbeatTimeout(time.Second * 12),
		HeartbeatExpiredTimeout(time.Second * 13),
		CheckElectionTimeout(time.Second * 14),
		WithLogger(NewLogger(nil)),
	}

	testNode1 := New("test1", "127.0.0.1", 1111)
	node := New(testNode1.ID(), testNode1.Addr(), testNode1.Port(), opts...)
	assert.Equal(t, time.Second*11, node.clientTimeout)
	assert.Equal(t, time.Second*12, node.heartbeatTimeout)
	assert.Equal(t, time.Second*13, node.heartbeatExpiredTimeout)
	assert.Equal(t, time.Second*14, node.electionCheckTimeout)
}

func TestNode(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", 1111)
	assert.Equal(t, "test1", testNode1.ID())
	assert.Equal(t, "127.0.0.1:1111", testNode1.AddrPort())
	assert.Equal(t, "127.0.0.1", testNode1.Addr())
	assert.Equal(t, 1111, testNode1.Port())
	assert.Equal(t, 0, testNode1.NumNodes())
	assert.Equal(t, time.Second*10, testNode1.clientTimeout)
	assert.Equal(t, time.Second*3, testNode1.heartbeatTimeout)
	assert.Equal(t, time.Second*5, testNode1.heartbeatExpiredTimeout)
	assert.Equal(t, time.Second*10, testNode1.electionCheckTimeout)

	testNode2 := New("test2", "127.0.0.2", 2222)
	assert.Equal(t, "test2", testNode2.ID())
	assert.Equal(t, "127.0.0.2:2222", testNode2.AddrPort())
	assert.Equal(t, "127.0.0.2", testNode2.Addr())
	assert.Equal(t, 2222, testNode2.Port())

	testNode3 := New("test3", "127.0.0.3", 3333)
	assert.Equal(t, "test3", testNode3.ID())
	assert.Equal(t, "127.0.0.3:3333", testNode3.AddrPort())
	assert.Equal(t, "127.0.0.3", testNode3.Addr())
	assert.Equal(t, 3333, testNode3.Port())

	testNode1.AddNode(testNode2, testNode3)
	assert.Equal(t, 2, testNode1.NumNodes())
	nodes := testNode1.Nodes()
	assert.Equal(t, testNode2.ID(), nodes[testNode2.ID()].ID())
	assert.Equal(t, testNode3.ID(), nodes[testNode3.ID()].ID())
}

func TestNodeStatus(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", 1111)
	testNode2 := New("test2", "127.0.0.2", 2222)
	testNode3 := New("test3", "127.0.0.3", 3333)

	assert.Nil(t, testNode1.getLeader())

	assert.Equal(t, pb.NodeStatus_Follower, testNode1.Status())
	assert.False(t, testNode1.isLeader())
	assert.True(t, testNode1.isFollower())

	testNode1.setStatus(pb.NodeStatus_Candidate)
	assert.Equal(t, pb.NodeStatus_Candidate, testNode1.Status())
	assert.False(t, testNode1.isLeader())
	assert.False(t, testNode1.isFollower())

	testNode1.setStatus(pb.NodeStatus_Leader)
	assert.Equal(t, pb.NodeStatus_Leader, testNode1.Status())
	assert.True(t, testNode1.isLeader())
	assert.False(t, testNode1.isFollower())
	assert.Equal(t, testNode1.ID(), testNode1.getLeader().ID())

	testNode1.AddNode(testNode2, testNode3)
	testNode1.newLeader(testNode3.ID())
	assert.True(t, testNode1.isFollower())
	assert.True(t, testNode2.isFollower())
	assert.Equal(t, testNode3.ID(), testNode1.getLeader().ID())

	testNode2.newLeader(testNode2.ID())
	assert.True(t, testNode2.isLeader())
}

func TestNodeVoted(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", 1111)

	assert.False(t, testNode1.isVoted())
	testNode1.setVoted()
	assert.True(t, testNode1.isVoted())
	testNode1.resetVoted()
	assert.False(t, testNode1.isVoted())

	assert.Equal(t, 0, testNode1.numVotes())
	testNode1.addVote()
	testNode1.addVote()
	assert.Equal(t, 2, testNode1.numVotes())
	testNode1.resetVotes()
	assert.Equal(t, 0, testNode1.numVotes())

	assert.Equal(t, int64(0), testNode1.getRound())
	testNode1.addRound()
	testNode1.addRound()
	assert.Equal(t, int64(2), testNode1.getRound())
	testNode1.setRound(12)
	assert.Equal(t, int64(12), testNode1.getRound())

	assert.True(t, testNode1.isFollower())
	assert.True(t, testNode1.agreeVote(3))
	testNode1.setVoted()
	assert.False(t, testNode1.agreeVote(3))
	testNode1.resetVoted()

	testNode1.setStatus(pb.NodeStatus_Candidate)
	assert.False(t, testNode1.agreeVote(3))
	assert.True(t, testNode1.agreeVote(13))
	assert.True(t, testNode1.isFollower())
	testNode1.resetVoted()

	testNode1.setStatus(pb.NodeStatus_Leader)
	assert.False(t, testNode1.agreeVote(3))
	assert.True(t, testNode1.agreeVote(13))
	assert.True(t, testNode1.isFollower())
}

func TestNodeHeartbeat(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", testNodeHeartbeatPort)
	testNode2 := New("test2", "127.0.0.2", testNodeHeartbeatPort)

	assert.True(t, testNode1.heartbeatTime.IsZero())
	assert.True(t, testNode1.heartbeatExpired())
	testNode1.setHeartbeat()
	assert.False(t, testNode1.heartbeatTime.IsZero())
	assert.Equal(t, time.Now().Format(time.DateTime), testNode1.heartbeatTime.Format(time.DateTime))
	assert.False(t, testNode1.heartbeatExpired())

	// By default, heartbeatExpiredTimeout is 5 seconds
	testNode1.heartbeatTime = time.Now().Add(-time.Second * 3)
	assert.False(t, testNode1.heartbeatExpired())
	testNode1.heartbeatTime = time.Now().Add(-time.Second * 6)
	assert.True(t, testNode1.heartbeatExpired())

	ctx := context.Background()
	chErr := make(chan error, 3)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	testNode1.sendHeartbeatWorker(ctx, wg, chErr, testNode2)
	wg.Wait()
	err := <-chErr
	assert.NotNil(t, err)

	lis, err := net.Listen("tcp", testNode2.AddrPort())
	require.Nil(t, err)
	grpcServer := grpc.NewServer()
	pb.RegisterNodeServer(grpcServer, newServer(testNode2))
	go func() {
		err = grpcServer.Serve(lis)
		require.Nil(t, err)
	}()

	chServStop := make(chan struct{})
	defer close(chServStop)
	go func() {
		<-chServStop
		grpcServer.Stop()
	}()

	testNode1.setStatus(pb.NodeStatus_Leader)
	wg.Add(1)
	testNode1.sendHeartbeatWorker(ctx, wg, chErr, testNode2)
	wg.Wait()
	err = <-chErr
	assert.Nil(t, err)

	chServStop <- struct{}{}
	testNode2.heartbeatTime = time.Now().Add(-time.Second * 6)
	wg.Add(1)
	testNode1.sendHeartbeatWorker(ctx, wg, chErr, testNode2)
	wg.Wait()
	err = <-chErr
	assert.NotNil(t, err)

	close(chErr)
}
