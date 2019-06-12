package rafthttp

import (
	"bytes"
	"github.com/ejunjsh/raftkv/pkg/raft/raftpb"
	"reflect"
	"testing"
)

func TestMessage(t *testing.T) {
	// Lower readBytesLimit to make test pass in restricted resources environment
	originalLimit := readBytesLimit
	readBytesLimit = 1000
	defer func() {
		readBytesLimit = originalLimit
	}()
	tests := []struct {
		msg       raftpb.Message
		encodeErr error
		decodeErr error
	}{
		{
			raftpb.Message{
				Type:    raftpb.MsgApp,
				From:    1,
				To:      2,
				Term:    1,
				LogTerm: 1,
				Index:   3,
				Entries: []raftpb.Entry{{Term: 1, Index: 4}},
			},
			nil,
			nil,
		},
		{
			raftpb.Message{
				Type: raftpb.MsgProp,
				From: 1,
				To:   2,
				Entries: []raftpb.Entry{
					{Data: []byte("some data")},
					{Data: []byte("some data")},
					{Data: []byte("some data")},
				},
			},
			nil,
			nil,
		},
		{
			raftpb.Message{
				Type: raftpb.MsgProp,
				From: 1,
				To:   2,
				Entries: []raftpb.Entry{
					{Data: bytes.Repeat([]byte("a"), int(readBytesLimit+10))},
				},
			},
			nil,
			ErrExceedSizeLimit,
		},
	}
	for i, tt := range tests {
		b := &bytes.Buffer{}
		enc := &messageEncoder{w: b}
		if err := enc.encode(&tt.msg); err != tt.encodeErr {
			t.Errorf("#%d: encode message error expected %v, got %v", i, tt.encodeErr, err)
			continue
		}
		dec := &messageDecoder{r: b}
		m, err := dec.decode()
		if err != tt.decodeErr {
			t.Errorf("#%d: decode message error expected %v, got %v", i, tt.decodeErr, err)
			continue
		}
		if err == nil {
			if !reflect.DeepEqual(m, tt.msg) {
				t.Errorf("#%d: message = %+v, want %+v", i, m, tt.msg)
			}
		}
	}
}