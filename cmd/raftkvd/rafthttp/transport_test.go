package rafthttp

import (
	"context"
	"github.com/ejunjsh/raftkv/pkg/raft"
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"github.com/ejunjsh/raftkv/pkg/testutil"
	"github.com/ejunjsh/raftkv/pkg/types"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestTransportSend tests that transport can send messages using correct
// underlying peer, and drop local or unknown-target messages.
func TestTransportSend(t *testing.T) {
	peer1 := newFakePeer()
	peer2 := newFakePeer()
	tr := &Transport{
		peers:  map[types.ID]Peer{types.ID(1): peer1, types.ID(2): peer2},
		Logger: testLogger,
	}
	wmsgsIgnored := []raftpb.Message{
		// bad local message
		{Type: raftpb.MsgBeat},
		// bad remote message
		{Type: raftpb.MsgProp, To: 3},
	}
	wmsgsTo1 := []raftpb.Message{
		// good message
		{Type: raftpb.MsgProp, To: 1},
		{Type: raftpb.MsgApp, To: 1},
	}
	wmsgsTo2 := []raftpb.Message{
		// good message
		{Type: raftpb.MsgProp, To: 2},
		{Type: raftpb.MsgApp, To: 2},
	}
	tr.Send(wmsgsIgnored)
	tr.Send(wmsgsTo1)
	tr.Send(wmsgsTo2)

	if !reflect.DeepEqual(peer1.msgs, wmsgsTo1) {
		t.Errorf("msgs to peer 1 = %+v, want %+v", peer1.msgs, wmsgsTo1)
	}
	if !reflect.DeepEqual(peer2.msgs, wmsgsTo2) {
		t.Errorf("msgs to peer 2 = %+v, want %+v", peer2.msgs, wmsgsTo2)
	}
}

func TestTransportCutMend(t *testing.T) {
	peer1 := newFakePeer()
	peer2 := newFakePeer()
	tr := &Transport{
		Logger: testLogger,
		peers:  map[types.ID]Peer{types.ID(1): peer1, types.ID(2): peer2},
	}

	tr.CutPeer(types.ID(1))

	wmsgsTo := []raftpb.Message{
		// good message
		{Type: raftpb.MsgProp, To: 1},
		{Type: raftpb.MsgApp, To: 1},
	}

	tr.Send(wmsgsTo)
	if len(peer1.msgs) > 0 {
		t.Fatalf("msgs expected to be ignored, got %+v", peer1.msgs)
	}

	tr.MendPeer(types.ID(1))

	tr.Send(wmsgsTo)
	if !reflect.DeepEqual(peer1.msgs, wmsgsTo) {
		t.Errorf("msgs to peer 1 = %+v, want %+v", peer1.msgs, wmsgsTo)
	}
}

func TestTransportAdd(t *testing.T) {
	tr := &Transport{
		streamRt: &roundTripperRecorder{},
		peers:    make(map[types.ID]Peer),
		Logger:   testLogger,
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})

	s, ok := tr.peers[types.ID(1)]
	if !ok {
		tr.Stop()
		t.Fatalf("senders[1] is nil, want exists")
	}

	// duplicate AddPeer is ignored
	tr.AddPeer(1, []string{"http://localhost:2380"})
	ns := tr.peers[types.ID(1)]
	if s != ns {
		t.Errorf("sender = %v, want %v", ns, s)
	}

	tr.Stop()
}

func TestTransportRemove(t *testing.T) {
	tr := &Transport{
		streamRt: &roundTripperRecorder{},
		peers:    make(map[types.ID]Peer),
		Logger:   testLogger,
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	tr.RemovePeer(types.ID(1))
	defer tr.Stop()

	if _, ok := tr.peers[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

func TestTransportUpdate(t *testing.T) {
	peer := newFakePeer()
	tr := &Transport{
		peers:  map[types.ID]Peer{types.ID(1): peer},
		Logger: testLogger,
	}
	u := "http://localhost:2380"
	tr.UpdatePeer(types.ID(1), []string{u})
	wurls := types.URLs(testutil.MustNewURLs(t, []string{"http://localhost:2380"}))
	if !reflect.DeepEqual(peer.peerURLs, wurls) {
		t.Errorf("urls = %+v, want %+v", peer.peerURLs, wurls)
	}
}

func TestTransportErrorc(t *testing.T) {
	errorc := make(chan error, 1)
	tr := &Transport{
		Raft:       &fakeRaft{},
		ErrorC:     errorc,
		streamRt:   newRespRoundTripper(http.StatusForbidden, nil),
		pipelineRt: newRespRoundTripper(http.StatusForbidden, nil),
		peers:      make(map[types.ID]Peer),
		Logger:     testLogger,
	}
	tr.AddPeer(1, []string{"http://localhost:2380"})
	defer tr.Stop()

	select {
	case <-errorc:
		t.Fatalf("received unexpected from errorc")
	case <-time.After(10 * time.Millisecond):
	}
	tr.peers[1].send(raftpb.Message{})

	select {
	case <-errorc:
	case <-time.After(1 * time.Second):
		t.Fatalf("cannot receive error from errorc")
	}
}

func TestSendingMsgApp(t *testing.T) {
	// member 1
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
		t.Fatalf("stream from 1 to 2 is not in work as expected")
	}

	data := make([]byte, 64)
	for i := 0; i < streamBufSize; i++ {
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
	//wait until all messages are received
	<-time.After(5 * time.Second)

	if r.count() != streamBufSize {
		t.Fatalf("count=%v,want %v", r.count(), streamBufSize)
	}
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
