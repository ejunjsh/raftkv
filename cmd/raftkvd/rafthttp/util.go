package rafthttp

import (
	"fmt"
	"github.com/ejunjsh/raftkv/pkg/transport"
	"github.com/ejunjsh/raftkv/pkg/types"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	errMemberRemoved  = fmt.Errorf("the member has been permanently removed from the cluster")
	errMemberNotFound = fmt.Errorf("member not found")
)

// NewListener returns a listener for raft message transfer between peers.
// It uses timeout listener to identify broken streams promptly.
func NewListener(u url.URL, tlsinfo *transport.TLSInfo) (net.Listener, error) {
	return transport.NewTimeoutListener(u.Host, u.Scheme, tlsinfo, ConnReadTimeout, ConnWriteTimeout)
}

// NewRoundTripper returns a roundTripper used to send requests
// to rafthttp listener of remote peers.
func NewRoundTripper(tlsInfo transport.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	// It uses timeout transport to pair with remote timeout listeners.
	// It sets no read/write timeout, because message in requests may
	// take long time to write out before reading out the response.
	return transport.NewTimeoutTransport(tlsInfo, dialTimeout, 0, 0)
}

// newStreamRoundTripper returns a roundTripper used to send stream requests
// to rafthttp listener of remote peers.
// Read/write timeout is set for stream roundTripper to promptly
// find out broken status, which minimizes the number of messages
// sent on broken connection.
func newStreamRoundTripper(tlsInfo transport.TLSInfo, dialTimeout time.Duration) (http.RoundTripper, error) {
	return transport.NewTimeoutTransport(tlsInfo, dialTimeout, ConnReadTimeout, ConnWriteTimeout)
}

// createPostRequest creates a HTTP POST request that sends raft message.
func createPostRequest(u url.URL, path string, body io.Reader, ct string, urls types.URLs, from, cid types.ID) *http.Request {
	uu := u
	uu.Path = path
	req, err := http.NewRequest("POST", uu.String(), body)
	if err != nil {
		log.Panicf("unexpected new request error (%v)", err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Server-From", from.String())
	req.Header.Set("X-Cluster-ID", cid.String())
	setPeerURLsHeader(req, urls)

	return req
}

// setPeerURLsHeader reports local urls for peer discovery
func setPeerURLsHeader(req *http.Request, urls types.URLs) {
	if urls == nil {
		// often not set in unit tests
		return
	}
	peerURLs := make([]string, urls.Len())
	for i := range urls {
		peerURLs[i] = urls[i].String()
	}
	req.Header.Set("X-PeerURLs", strings.Join(peerURLs, ","))
}

// checkPostResponse checks the response of the HTTP POST request that sends
// raft message.
func checkPostResponse(resp *http.Response, body []byte, req *http.Request, to types.ID) error {
	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		switch strings.TrimSuffix(string(body), "\n") {
		case errClusterIDMismatch.Error():
			log.Println("request sent was ignored (cluster ID mismatch: remote[%s]=%s, local=%s)",
				to, resp.Header.Get("X-Cluster-ID"), req.Header.Get("X-Cluster-ID"))
			return errClusterIDMismatch
		default:
			return fmt.Errorf("unhandled error %q when precondition failed", string(body))
		}
	case http.StatusForbidden:
		return errMemberRemoved
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}

// reportCriticalError reports the given error through sending it into
// the given error channel.
// If the error channel is filled up when sending error, it drops the error
// because the fact that error has happened is reported, which is
// good enough.
func reportCriticalError(err error, errc chan<- error) {
	select {
	case errc <- err:
	default:
	}
}
