package rafthttp

import (
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"github.com/ejunjsh/raftkv/pkg/types"
	"go.uber.org/zap"
)

type remote struct {
	lg       *zap.Logger
	localID  types.ID
	id       types.ID
	status   *peerStatus
	pipeline *pipeline
}

func startRemote(tr *Transport, urls types.URLs, id types.ID) *remote {
	picker := newURLPicker(urls)
	status := newPeerStatus(tr.Logger, tr.ID, id)
	pipeline := &pipeline{
		peerID: id,
		tr:     tr,
		picker: picker,
		status: status,
		raft:   tr.Raft,
		errorc: tr.ErrorC,
	}
	pipeline.start()

	return &remote{
		lg:       tr.Logger,
		localID:  tr.ID,
		id:       id,
		status:   status,
		pipeline: pipeline,
	}
}

func (g *remote) send(m raftpb.Message) {
	select {
	case g.pipeline.msgc <- m:
	default:
		if g.status.isActive() {
			g.lg.Warn(
				"dropped internal Raft message since sending buffer is full (overloaded network)",
				zap.String("message-type", m.Type.String()),
				zap.String("local-member-id", g.localID.String()),
				zap.String("from", types.ID(m.From).String()),
				zap.String("remote-peer-id", g.id.String()),
				zap.Bool("remote-peer-active", g.status.isActive()),
			)
		} else {
			g.lg.Warn(
				"dropped Raft message since sending buffer is full (overloaded network)",
				zap.String("message-type", m.Type.String()),
				zap.String("local-member-id", g.localID.String()),
				zap.String("from", types.ID(m.From).String()),
				zap.String("remote-peer-id", g.id.String()),
				zap.Bool("remote-peer-active", g.status.isActive()),
			)
		}
	}
}

func (g *remote) stop() {
	g.pipeline.stop()
}

func (g *remote) Pause() {
	g.stop()
}

func (g *remote) Resume() {
	g.pipeline.start()
}
