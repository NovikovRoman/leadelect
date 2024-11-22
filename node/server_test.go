//go:build !race

package node

import (
	"context"
	"net"
	"testing"

	pb "github.com/NovikovRoman/leadelect/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

var (
	testServerPort = 2222
)

func TestServerVote(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", testServerPort)
	testNode2 := New("test2", "127.0.0.2", testServerPort)

	lis, err := net.Listen("tcp", testNode1.AddrPort())
	require.Nil(t, err)
	grpcServer := grpc.NewServer()
	pb.RegisterNodeServer(grpcServer, newServer(testNode1))
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

	ctx := context.Background()
	status, err := testNode2.grpcClient.vote(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.True(t, status.Vote)

	// The voting round is below
	testNode1.round = 10
	status, err = testNode2.grpcClient.vote(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.Less(t, testNode2.round, testNode1.round)
	assert.False(t, status.Vote)

	// The node has already cast its vote
	testNode1.round = 0
	testNode1.voted = true
	status, err = testNode2.grpcClient.vote(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.False(t, status.Vote)

	// The node is not a follower
	testNode1.setStatus(pb.NodeStatus_Leader)
	status, err = testNode2.grpcClient.vote(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.False(t, status.Vote)

	// The node is not a follower
	testNode1.setStatus(pb.NodeStatus_Candidate)
	status, err = testNode2.grpcClient.vote(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.False(t, status.Vote)

	chServStop <- struct{}{}
}

func Test_ServerStatus(t *testing.T) {
	testNode1 := New("test1", "127.0.0.1", testServerPort+1)
	testNode2 := New("test2", "127.0.0.2", testServerPort+1)

	lis, err := net.Listen("tcp", testNode1.AddrPort())
	require.Nil(t, err)

	_, err = net.Listen("tcp", testNode1.AddrPort())
	require.NotNil(t, err)

	grpcServer := grpc.NewServer()
	pb.RegisterNodeServer(grpcServer, newServer(testNode1))
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

	ctx := context.Background()
	status, err := testNode2.grpcClient.status(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.Equal(t, pb.NodeStatus_Follower, status.Status)

	testNode1.setStatus(pb.NodeStatus_Candidate)
	status, err = testNode2.grpcClient.status(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.Equal(t, pb.NodeStatus_Candidate, status.Status)

	testNode1.setStatus(pb.NodeStatus_Leader)
	status, err = testNode2.grpcClient.status(ctx, testNode1.AddrPort())
	require.Nil(t, err)
	assert.Equal(t, pb.NodeStatus_Leader, status.Status)

	chServStop <- struct{}{}
}
