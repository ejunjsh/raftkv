package rafthttp

import (
	"context"
	"github.com/ejunjsh/raftkv/pkg/raft"
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"github.com/ejunjsh/raftkv/pkg/types"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func BenchmarkSendingMsgApp(b *testing.B) {
	// member 1
	streamBufSize = b.N
	tr := &Transport{
		ID:        types.ID(1),
		ClusterID: types.ID(1),
		Raft:      &fakeRaft{},
		Logger:    testLogger,
	}
	tr.Start()
	srv := httptest.NewServer(tr.Handler())
	defer srv.Close()

	// member 2
	r := &countRaft{}
	tr2 := &Transport{
		ID:        types.ID(2),
		ClusterID: types.ID(1),
		Raft:      r,
		Logger:    testLogger,
	}
	tr2.Start()
	srv2 := httptest.NewServer(tr2.Handler())
	defer srv2.Close()

	tr.AddPeer(types.ID(2), []string{srv2.URL})
	defer tr.Stop()
	tr2.AddPeer(types.ID(1), []string{srv.URL})
	defer tr2.Stop()
	if !waitStreamWorking(tr.Get(types.ID(2)).(*peer)) {
		b.Fatalf("stream from 1 to 2 is not in work as expected")
	}

	b.ReportAllocs()
	b.SetBytes(64)

	b.ResetTimer()
	data := make([]byte, 64)
	for i := 0; i < b.N; i++ {
		tr.Send([]raftpb.Message{
			{
				Type:  raftpb.MsgApp,
				From:  1,
				To:    2,
				Index: uint64(i),
				Entries: []raftpb.Entry{
					{
						Index: uint64(i + 1),
						Data:  data,
					},
				},
			},
		})
	}
	// wait until all messages are received by the target raft
	for r.count() != b.N {
		time.Sleep(time.Millisecond)
	}
	b.StopTimer()
}

type countRaft struct {
	mu  sync.Mutex
	cnt int
}

func (r *countRaft) Process(ctx context.Context, m raftpb.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cnt++
	return nil
}

func (r *countRaft) IsIDRemoved(id uint64) bool { return false }

func (r *countRaft) ReportUnreachable(id uint64) {}

func (r *countRaft) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}

func (r *countRaft) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cnt
}

func waitStreamWorking(p *peer) bool {
	for i := 0; i < 1000; i++ {
		time.Sleep(time.Millisecond)
		if _, ok := p.msgAppV2Writer.writec(); !ok {
			continue
		}
		if _, ok := p.writer.writec(); !ok {
			continue
		}
		return true
	}
	return false
}
