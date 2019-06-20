package rafthttp

const (
	msgTypeLinkHeartbeat uint8 = 0
	msgTypeAppEntries    uint8 = 1
	msgTypeApp           uint8 = 2

	msgAppV2BufSize = 1024 * 1024
)
