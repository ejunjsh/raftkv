#!/usr/bin/env bash

set -e

go install ./cmd/raftkvd
go test -v ./cmd/raftkvd/rafthttp
go test -v ./cmd/raftkvd
go test -v ./pkg/raft
go test -v ./pkg/crc
go test -v ./pkg/fileutil
go test -v ./pkg/httputil
go test -v ./pkg/ioutil
go test -v ./pkg/snap
go test -v ./pkg/testutil
go test -v ./pkg/transport
go test -v ./pkg/types
go test -v ./pkg/wal -bench ^BenchmarkWrite100