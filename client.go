package sdk

import (
	"encoding/json"
	"net"
	"os"
	"sync"

	"github.com/google/uuid"
)

type RegistryKey struct {
	ClientID  string
	RequestID uint64
}

type Client struct {
	id string

	conn            net.Conn
	nextID          uint64
	pendingRequests map[RegistryKey]chan []byte

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
		pendingRequests: make(map[RegistryKey]chan []byte),

		mu: &sync.Mutex{},
	}

	errChan := make(chan error)
	go func() {
		err = client.listen()
		errChan <- err
	}()

	err = <-errChan
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) listen() error {
	decoder := json.NewDecoder(c.conn)

	for {
		var resp response
		if err := decoder.Decode(&resp); err != nil {
			return err
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
			ch <- resp.Data
			close(ch)
		}
	}
}
