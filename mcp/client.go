package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

type response struct {
	JSONRPC string           `json:"jsonrpc"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
	ID      any              `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("jsonrpc: code: %d, message: %s", e.Code, e.Message)
}

type client struct {
	encoder   *json.Encoder
	decoder   *json.Decoder
	nextID    int64
	idMu      sync.Mutex
	pending   map[int64]chan *response
	pendingMu sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func newClient(r io.Reader, w io.Writer) *client {
	ctx, cancel := context.WithCancel(context.Background())
	return &client{
		encoder: json.NewEncoder(w),
		decoder: json.NewDecoder(r),
		nextID:  1,
		pending: make(map[int64]chan *response),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// call sends a JSON-RPC request and blocks until the response is received.
func (c *client) call(ctx context.Context, method string, params any, result any) error {
	c.idMu.Lock()
	id := c.nextID
	c.nextID++
	c.idMu.Unlock()

	ch := make(chan *response, 1)

	c.pendingMu.Lock()
	select {
	case <-c.ctx.Done():
		c.pendingMu.Unlock()
		return fmt.Errorf("jsonrpc: client is closed: %w", c.ctx.Err())
	default:
	}
	c.pending[id] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if err := c.encoder.Encode(request{JSONRPC: "2.0", Method: method, Params: params, ID: id}); err != nil {
		return fmt.Errorf("jsonrpc: failed to send request: %w", err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("jsonrpc: call cancelled: %w", ctx.Err())
	case <-c.ctx.Done():
		return fmt.Errorf("jsonrpc: client is closing: %w", c.ctx.Err())
	case resp := <-ch:
		if resp == nil {
			return fmt.Errorf("jsonrpc: call for ID %d aborted due to shutdown", id)
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(*resp.Result, result); err != nil {
				return fmt.Errorf("jsonrpc: failed to unmarshal result: %w", err)
			}
		}
		return nil
	}
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *client) notify(ctx context.Context, method string, params any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := c.encoder.Encode(request{JSONRPC: "2.0", Method: method, Params: params}); err != nil {
		return fmt.Errorf("jsonrpc: failed to send notification: %w", err)
	}
	return nil
}

// listen reads incoming JSON-RPC messages and dispatches responses to pending calls.
// Notifications from the server are silently ignored.
func (c *client) listen() error {
	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
		}

		var msg struct {
			Method string           `json:"method,omitempty"`
			ID     any              `json:"id,omitempty"`
			Result *json.RawMessage `json:"result,omitempty"`
			Error  *rpcError        `json:"error,omitempty"`
		}

		if err := c.decoder.Decode(&msg); err != nil {
			if c.ctx.Err() != nil {
				return c.ctx.Err()
			}
			return fmt.Errorf("jsonrpc: decode error: %w", err)
		}

		// TODO: Server notification or request — ignore.
		if msg.Method != "" {
			continue
		}

		if msg.ID == nil {
			continue
		}

		// JSON numbers decode as float64.
		idFloat, ok := msg.ID.(float64)
		if !ok {
			continue
		}
		id := int64(idFloat)

		c.pendingMu.Lock()
		ch, found := c.pending[id]
		c.pendingMu.Unlock()

		if found && ch != nil {
			ch <- &response{Result: msg.Result, Error: msg.Error, ID: msg.ID}
		}
	}
}

// close cancels the listener, waits for it to exit, and cleans up pending calls.
func (c *client) close() error {
	c.cancel()
	c.wg.Wait()

	c.pendingMu.Lock()
	for id, ch := range c.pending {
		if ch != nil {
			select {
			case ch <- nil:
			default:
			}
			close(ch)
		}
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
	return nil
}
