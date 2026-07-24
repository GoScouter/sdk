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
    ClientID string
    RequestID string
}

type Client struct {
    id string
    conn net.Conn
    mu *sync.Mutex

    pendingRequests map[RegistryKey]chan []byte
}

func NewClient() (*Client, error) {
    conn, err := net.Dial("unix", os.Getenv("GS_SOCKET"))
    if err != nil {
        return nil, err
    }

    client := Client{
        id: uuid.NewString(),
        conn: conn,
        pendingRequests: make(map[RegistryKey]chan []byte),
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

    return &client, nil
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
            ClientID: resp.ClientID,
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

type RequestType string

const (
    TypeAsk = "ask"
)

type Request struct {
    ID string `json:"id"`
    ClientID string `json:"client_id"`
    Type RequestType `json:"type"`
    Data json.RawMessage `json:"data"`
}

type AskRequest ModuleNamespace

type Response struct {
    ClientID string `json:"client_id"`
    RequestID string `json:"request_id"`
    Data json.RawMessage `json:"data"`
}

func (c *Client) AskModule(module Module, namespace ModuleNamespace) (string, error) {
    c.mu.Lock()

    responseChan := make(chan []byte, 1)
    key := RegistryKey{
        ClientID: c.id,
        RequestID: uuid.NewString(),
    }

    c.pendingRequests[key] = responseChan
    c.mu.Unlock()

    defer func() {
        c.mu.Lock()
        delete(c.pendingRequests, key)
        c.mu.Unlock()
    }()

    askRequest, err := json.Marshal(AskRequest{
        Name: namespace.Name,
        Author: namespace.Author,
    })
    if err != nil {
        return "", err
    }

    req, err := json.Marshal(Request{
        ID: key.RequestID,
        ClientID: key.ClientID,
        Type: TypeAsk,
        Data: askRequest,
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
        return "", fmt.Errorf("request %s for client %s, timed out waiting for module response", key.RequestID, key.ClientID)
    }
}
