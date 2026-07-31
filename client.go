package sdk

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// askTimeout bounds a single AskModule call so a runner that never answers
// cannot park a Scout indefinitely.
const askTimeout = 8 * time.Second

type RegistryKey struct {
	ClientID  string
	RequestID uint64
}

type Client struct {
	id string

	conn            net.Conn
	nextID          uint64
	pendingRequests map[RegistryKey]chan *Response

	mu *sync.Mutex
}

func NewClient() (*Client, error) {
	conn, err := net.Dial("unix", os.Getenv("GS_SOCKET"))
	if err != nil {
		return nil, err
	}

	client := &Client{
		id: uuid.NewString(),

		conn:            conn,
		nextID:          0,
		pendingRequests: make(map[RegistryKey]chan *Response),

		mu: &sync.Mutex{},
	}

	// listen runs for the life of the connection. Waiting on it here would
	// block until the socket breaks, which is to say forever.
	go client.listen()

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) listen() {
	decoder := json.NewDecoder(c.conn)

	for {
		var resp Response
		if err := decoder.Decode(&resp); err != nil {
			c.fail()
			return
		}

		c.mu.Lock()
		key := RegistryKey{
			ClientID:  resp.ClientID,
			RequestID: resp.RequestID,
		}
		ch, exists := c.pendingRequests[key]
		if exists {
			delete(c.pendingRequests, key)
		}
		c.mu.Unlock()

		if exists {
			ch <- &resp // buffered, and the only send on this channel
			close(ch)
		}
	}
}

func (c *Client) fail() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, ch := range c.pendingRequests {
		delete(c.pendingRequests, key)
		close(ch)
	}
}

func (c *Client) AskModule(namespace ModuleNamespace) (string, error) {
	c.mu.Lock()

	responseChan := make(chan *Response, 1)
	key := RegistryKey{
		ClientID:  c.id,
		RequestID: c.nextID,
	}

	c.nextID += 1
	c.pendingRequests[key] = responseChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pendingRequests, key)
		c.mu.Unlock()
	}()

	askRequest, err := json.Marshal(AskRequest(ModuleNamespace{
		Name:   namespace.Name,
		Author: namespace.Author,
	}))
	if err != nil {
		return "", err
	}

	req, err := json.Marshal(Request{
		RequestID: key.RequestID,
		ClientID:  key.ClientID,
		Type:      MethodAsk,
		Data:      askRequest,
	})
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	_, err = c.conn.Write(req)
	c.mu.Unlock()
	if err != nil {
		return "", err
	}

	select {
	case resp, ok := <-responseChan:
		if !ok {
			return "", fmt.Errorf("sdk: connection to the runner closed while asking %s:%s", namespace.Author, namespace.Name)
		}
		if resp.Error != "" {
			return "", fmt.Errorf("sdk: ask %s:%s: %s", namespace.Author, namespace.Name, resp.Error)
		}
		return string(resp.Data), nil

	case <-time.After(askTimeout):
		return "", fmt.Errorf("request %d for client %s, timed out waiting for module response", key.RequestID, key.ClientID)
	}
}
