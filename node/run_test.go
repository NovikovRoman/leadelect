//go:build !race

package node

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"testing"

	pb "github.com/NovikovRoman/leadelect/grpc"
	"github.com/stretchr/testify/assert"
)

var (
	testRunPort = 10101
)

func TestRun_serverRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testNode1 := New("test_bug", "127.0.0", 101010)
	defer cancel()
	err := testNode1.serverRun(ctx)
	assert.NotNil(t, err)

	testNode1 = New("test1", "127.0.0.1", testRunPort)
	go func() {
		err := testNode1.serverRun(ctx)
		assert.Nil(t, err)
	}()

	testNode2 := New("test2", "127.0.0.2", testRunPort)
	status, err := testNode2.grpcClient.status(ctx, testNode1.AddrPort())
	assert.Nil(t, err)
	assert.Equal(t, pb.NodeStatus_Follower, status.Status)
}

func TestRun_initNode(t *testing.T) {
	portOffset := 1
	testNode1 := New("test1", "127.0.0.1", testRunPort+portOffset)
	testNode2 := New("test2", "127.0.0.2", testRunPort+portOffset)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := testNode1.initNode(ctx, testNode2)

	assert.NotNil(t, r.err)
	assert.Nil(t, r.resp)

	go func() {
		err := testNode2.serverRun(ctx)
		assert.Nil(t, err)
	}()

	r = testNode1.initNode(ctx, testNode2)
	assert.Nil(t, r.err)
	assert.Equal(t, pb.NodeStatus_Follower, r.resp.Status)

	testNode2.setStatus(pb.NodeStatus_Leader)
	r = testNode1.initNode(ctx, testNode2)
	assert.Nil(t, r.err)
	assert.Equal(t, pb.NodeStatus_Leader, r.resp.Status)
}

func TestRun_votingNodes(t *testing.T) {
	portOffset := 2

	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	l := NewLogger(slog.NewTextHandler(w, nil))

	testNode1 := New("test1", "127.0.0.1", testRunPort+portOffset)
	testNode2 := New("test2", "127.0.0.2", testRunPort+portOffset)
	testNode3 := New("test3", "127.0.0.3", testRunPort+portOffset)
	testNode1.logger = l
	testNode2.logger = l
	testNode3.logger = l
	testNode1.AddNode(testNode2, testNode3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := testNode2.serverRun(ctx)
		assert.Nil(t, err)
	}()

	maxRound := testNode1.votingNodes(ctx)
	assert.Equal(t, int64(0), maxRound)
	testNode1.resetVotes()
	testNode2.resetVoted()

	testNode2.setStatus(pb.NodeStatus_Leader)
	maxRound = testNode1.votingNodes(ctx)
	assert.Equal(t, int64(0), maxRound)

	testNode2.setStatus(pb.NodeStatus_Follower)
	testNode2.round = 10
	testNode1.setStatus(pb.NodeStatus_Candidate)
	maxRound = testNode1.votingNodes(ctx)
	assert.Equal(t, int64(10), maxRound)

	testNode2.round = 5
	testNode3.round = 10 // server not working
	maxRound = testNode1.votingNodes(ctx)
	assert.Equal(t, int64(5), maxRound)

	w.Flush()
}
