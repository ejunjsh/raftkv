package rafthttp

import (
	"context"
	"errors"
	"github.com/ejunjsh/raftkv/pkg/raft"
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"go.uber.org/zap"
	"net/http"
)

func (t *roundTripperBlocker) RoundTrip(req *http.Request) (*http.Response, error) {
	c := make(chan struct{}, 1)
	t.mu.Lock()
	t.cancel[req] = c
	t.mu.Unlock()
	ctx := req.Context()
	select {
	case <-t.unblockc:
		return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReadCloser{}}, nil
	case <-ctx.Done():
		return nil, errors.New("request canceled")
	case <-c:
		return nil, errors.New("request canceled")
	}
}

var testLogger = zap.NewExample()

type fakeRaft struct {
	recvc     chan<- raftpb.Message
	err       error
	removedID uint64
}

func (p *fakeRaft) Process(ctx context.Context, m raftpb.Message) error {
	select {
	case p.recvc <- m:
	default:
	}
	return p.err
}

func (p *fakeRaft) IsIDRemoved(id uint64) bool { return id == p.removedID }

func (p *fakeRaft) ReportUnreachable(id uint64) {}

func (p *fakeRaft) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}
