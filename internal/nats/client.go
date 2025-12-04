package nats

import (
	"encoding/json"
	"fmt"
	"time"

	nc "github.com/nats-io/nats.go"
)

// Message represents a NATS message with subject, reply, and data
type Message struct {
	Subject string
	Reply   string
	Data    []byte
}

// Client wraps a NATS connection with convenience methods
type Client struct {
	conn *nc.Conn
}

// NewClient creates a new NATS client with reconnect handling
func NewClient(url string) (*Client, error) {
	opts := []nc.Option{
		nc.ReconnectWait(2 * time.Second),
		nc.MaxReconnects(-1), // Reconnect indefinitely
		nc.DisconnectErrHandler(func(conn *nc.Conn, err error) {
			if err != nil {
				fmt.Printf("[NATS] Disconnected: %v\n", err)
			}
		}),
		nc.ReconnectHandler(func(conn *nc.Conn) {
			fmt.Printf("[NATS] Reconnected to %s\n", conn.ConnectedUrl())
		}),
		nc.ClosedHandler(func(conn *nc.Conn) {
			fmt.Println("[NATS] Connection closed")
		}),
	}

	conn, err := nc.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Publish publishes data to a subject
func (c *Client) Publish(subject string, data []byte) error {
	if err := c.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}
	return nil
}

// PublishJSON publishes a JSON-encoded message to a subject
func (c *Client) PublishJSON(subject string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return c.Publish(subject, data)
}

// Subscribe creates an asynchronous subscription
func (c *Client) Subscribe(subject string, handler func(*Message)) (*nc.Subscription, error) {
	sub, err := c.conn.Subscribe(subject, func(msg *nc.Msg) {
		handler(&Message{
			Subject: msg.Subject,
			Reply:   msg.Reply,
			Data:    msg.Data,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}
	return sub, nil
}

// Request sends a request and waits for a reply
func (c *Client) Request(subject string, data []byte, timeout time.Duration) (*Message, error) {
	msg, err := c.conn.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", subject, err)
	}
	return &Message{
		Subject: msg.Subject,
		Reply:   msg.Reply,
		Data:    msg.Data,
	}, nil
}

// RequestJSON sends a JSON request and decodes the JSON response
func (c *Client) RequestJSON(subject string, req interface{}, resp interface{}, timeout time.Duration) error {
	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	msg, err := c.Request(subject, reqData, timeout)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(msg.Data, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// QueueSubscribe creates a load-balanced queue subscription
func (c *Client) QueueSubscribe(subject, queue string, handler func(*Message)) (*nc.Subscription, error) {
	sub, err := c.conn.QueueSubscribe(subject, queue, func(msg *nc.Msg) {
		handler(&Message{
			Subject: msg.Subject,
			Reply:   msg.Reply,
			Data:    msg.Data,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to queue subscribe to %s: %w", subject, err)
	}
	return sub, nil
}

// Flush flushes the buffered data to the server
func (c *Client) Flush() error {
	if err := c.conn.Flush(); err != nil {
		return fmt.Errorf("flush failed: %w", err)
	}
	return nil
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

// RawConn returns the underlying NATS connection for advanced use cases
func (c *Client) RawConn() *nc.Conn {
	return c.conn
}
