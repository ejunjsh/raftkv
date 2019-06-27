package rafthttp

import (
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"github.com/ejunjsh/raftkv/pkg/testutil"
	"github.com/ejunjsh/raftkv/pkg/types"
	"net/http"
	"reflect"
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
