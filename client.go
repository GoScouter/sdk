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
		var resp Response
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

func (c *Client) AskModule(namespace ModuleNamespace) (string, error) {
	c.mu.Lock()

	responseChan := make(chan []byte, 1)
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

	askRequest, err := json.Marshal(askRequest(ModuleNamespace{
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
	case rawData := <-responseChan:
		return string(rawData), nil

	case <-time.After(8 * time.Second):
		return "", fmt.Errorf("request %d for client %s, timed out waiting for module response", key.RequestID, key.ClientID)
	}
}
