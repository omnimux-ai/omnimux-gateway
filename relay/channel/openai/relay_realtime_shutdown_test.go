package openai

import (
	"context"
	"net"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A transport that deterministically models a peer no longer reading, without
// relying on platform socket buffer sizes or a flood of oversized messages.
type realtimeBlockedWriteConn struct {
	net.Conn
	expirePendingDeadline atomic.Bool
	armed                 atomic.Bool
	entered               chan struct{}
	closed                chan struct{}
	released              chan struct{}
	readStopped           chan struct{}
	readOnce              sync.Once
	enterOnce             sync.Once
	closeOnce             sync.Once
}

func (c *realtimeBlockedWriteConn) Write(p []byte) (int, error) {
	if c.armed.Load() {
		c.enterOnce.Do(func() { close(c.entered) })
		select {
		case <-c.closed:
			return 0, net.ErrClosed
		case <-c.released:
		}
	}
	return c.Conn.Write(p)
}

// Simulate time spent in caller processing without sleeps: any inherited
// finite deadline is expired when gorilla applies it on the next write.
func (c *realtimeBlockedWriteConn) SetWriteDeadline(deadline time.Time) error {
	if c.expirePendingDeadline.Load() && !deadline.IsZero() {
		deadline = time.Now().Add(-time.Second)
	}
	return c.Conn.SetWriteDeadline(deadline)
}

func (c *realtimeBlockedWriteConn) SetReadDeadline(deadline time.Time) error {
	err := c.Conn.SetReadDeadline(deadline)
	c.readOnce.Do(func() { close(c.readStopped) })
	return err
}

func (c *realtimeBlockedWriteConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestOpenaiRealtimeShutdown(t *testing.T) {
	for _, cause := range []string{"request cancellation", "upstream close", "client close", "blocked client write", "in-flight client write", "caller processing delay"} {
		t.Run(cause, func(t *testing.T) {
			var transport *realtimeBlockedWriteConn
			relayClient, client := realtimeDebtConnectionPair(t, func(conn net.Conn) net.Conn {
				transport = &realtimeBlockedWriteConn{Conn: conn, entered: make(chan struct{}), closed: make(chan struct{}), released: make(chan struct{}), readStopped: make(chan struct{})}
				return transport
			})
			upstream, relayTarget := realtimeDebtConnectionPair(t)
			requestContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", "/v1/realtime", nil).WithContext(requestContext)
			info := &relaycommon.RelayInfo{ClientWs: relayClient, TargetWs: relayTarget}
			result := make(chan realtimeDebtResult, 1)
			go func() {
				apiErr, usage := OpenaiRealtimeHandler(ctx, info)
				result <- realtimeDebtResult{err: apiErr, usage: usage}
			}()

			switch cause {
			case "blocked client write", "in-flight client write":
				transport.armed.Store(true)
				require.NoError(t, upstream.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created"}`)))
				select {
				case <-transport.entered:
				case <-time.After(3 * time.Second):
					t.Fatal("upstream frame never reached client writer")
				}
				cancel()
				if cause == "in-flight client write" {
					select {
					case <-transport.readStopped:
					case <-time.After(3 * time.Second):
						t.Fatal("shutdown never interrupted the client reader")
					}
					transport.armed.Store(false)
					close(transport.released)
				}
			case "request cancellation":
				cancel()
			case "upstream close", "caller processing delay":
				require.NoError(t, upstream.Close())
			case "client close":
				require.NoError(t, client.Close())
			}
			select {
			case got := <-result:
				assert.Nil(t, got.err)
				require.NotNil(t, got.usage)
				assert.Zero(t, got.usage.TotalTokens)
			case <-time.After(3 * time.Second):
				t.Fatal("relay did not finish shutdown")
			}

			if cause == "request cancellation" || cause == "upstream close" || cause == "in-flight client write" || cause == "caller processing delay" {
				// Returning must retain a writable healthy client for the caller's native
				// terminal envelope, even though its reader has been interrupted.
				if cause == "caller processing delay" {
					transport.expirePendingDeadline.Store(true)
				}
				helper.WssError(ctx, relayClient, types.OpenAIError{Message: "terminal", Type: "test"})
				require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
				if cause == "in-flight client write" {
					_, frame, err := client.ReadMessage()
					require.NoError(t, err)
					assert.Contains(t, string(frame), `"type":"session.created"`)
				}
				_, frame, err := client.ReadMessage()
				require.NoError(t, err)
				assert.Contains(t, string(frame), `"type":"error"`)
				assert.Contains(t, string(frame), `"message":"terminal"`)
			}
			if cause == "blocked client write" {
				select {
				case <-transport.closed:
				default:
					t.Fatal("blocked writer was not interrupted")
				}
			}
		})
	}
}
