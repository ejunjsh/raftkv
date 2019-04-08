package raft

import (
	pb "github.com/ejunjsh/kv/pkg/raft/raftpb"
)

func IsEmptySnap(sp pb.Snapshot) bool {
	return sp.Metadata.Index == 0
}
