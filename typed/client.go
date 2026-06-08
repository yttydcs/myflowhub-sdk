package typed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-sdk/await"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

var (
	ErrAwaitClientRequired   = errors.New("await client is required")
	ErrSourceIDRequired      = errors.New("source_id is required")
	ErrTargetIDRequired      = errors.New("target_id is required")
	ErrResponseActionMissing = errors.New("response action is required")
	ErrResponseDataEmpty     = errors.New("response data is empty")
)

// Options contains the default routing identifiers used by typed clients.
// TargetID must be a real node id; TargetID=0 is child broadcast in Core, not upstream routing.
type Options struct {
	SourceID uint32
	TargetID uint32
}

// Client is a typed subprotocol facade over await.Client.
type Client struct {
	base *await.Client
	opts Options
}

// New creates a typed client facade over an already constructed await client.
func New(base *await.Client, opts Options) *Client {
	return &Client{base: base, opts: opts}
}

// Options returns the current routing defaults.
func (c *Client) Options() Options {
	if c == nil {
		return Options{}
	}
	return c.opts
}

// WithOptions returns a shallow copy using new routing defaults.
func (c *Client) WithOptions(opts Options) *Client {
	if c == nil {
		return &Client{opts: opts}
	}
	clone := *c
	clone.opts = opts
	return &clone
}

// WithSource returns a shallow copy using another source node id.
func (c *Client) WithSource(sourceID uint32) *Client {
	opts := c.Options()
	opts.SourceID = sourceID
	return c.WithOptions(opts)
}

// WithTarget returns a shallow copy using another target node id.
func (c *Client) WithTarget(targetID uint32) *Client {
	opts := c.Options()
	opts.TargetID = targetID
	return c.WithOptions(opts)
}

func (c *Client) Management() *ManagementClient {
	return &ManagementClient{client: c}
}

func (c *Client) Auth() *AuthClient {
	return &AuthClient{client: c}
}

func (c *Client) VarStore() *VarStoreClient {
	return &VarStoreClient{client: c}
}

func (c *Client) TopicBus() *TopicBusClient {
	return &TopicBusClient{client: c}
}

func (c *Client) validateRoute() error {
	if err := c.validateBase(); err != nil {
		return err
	}
	if c.opts.SourceID == 0 {
		return ErrSourceIDRequired
	}
	if c.opts.TargetID == 0 {
		return ErrTargetIDRequired
	}
	return nil
}

func (c *Client) validateBase() error {
	if c == nil || c.base == nil {
		return ErrAwaitClientRequired
	}
	return nil
}

func (c *Client) header(subProto uint8, payloadLen int) *header.HeaderTcp {
	return headerWithOptions(c.opts, subProto, payloadLen)
}

func headerWithOptions(opts Options, subProto uint8, payloadLen int) *header.HeaderTcp {
	return (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(subProto).
		WithSourceID(opts.SourceID).
		WithTargetID(opts.TargetID).
		WithPayloadLength(uint32(payloadLen)).(*header.HeaderTcp)
}

func sendAndDecode[T any](ctx context.Context, c *Client, subProto uint8, action string, req any, respAction string) (T, error) {
	var out T
	if err := c.validateRoute(); err != nil {
		return out, err
	}
	respAction = strings.TrimSpace(respAction)
	if respAction == "" {
		return out, ErrResponseActionMissing
	}
	payload, err := transport.EncodeMessage(action, req)
	if err != nil {
		return out, err
	}
	resp, err := c.base.SendAndAwait(ctx, c.header(subProto, len(payload)), payload, respAction)
	if err != nil {
		return out, err
	}
	if len(resp.Message.Data) == 0 {
		return out, ErrResponseDataEmpty
	}
	if err := json.Unmarshal(resp.Message.Data, &out); err != nil {
		return out, fmt.Errorf("decode %s data: %w", respAction, err)
	}
	return out, nil
}

func sendAndDecodeWithOptions[T any](ctx context.Context, c *Client, opts Options, subProto uint8, action string, req any, respAction string) (T, error) {
	var out T
	if err := c.validateBase(); err != nil {
		return out, err
	}
	respAction = strings.TrimSpace(respAction)
	if respAction == "" {
		return out, ErrResponseActionMissing
	}
	payload, err := transport.EncodeMessage(action, req)
	if err != nil {
		return out, err
	}
	resp, err := c.base.SendAndAwait(ctx, headerWithOptions(opts, subProto, len(payload)), payload, respAction)
	if err != nil {
		return out, err
	}
	if len(resp.Message.Data) == 0 {
		return out, ErrResponseDataEmpty
	}
	if err := json.Unmarshal(resp.Message.Data, &out); err != nil {
		return out, fmt.Errorf("decode %s data: %w", respAction, err)
	}
	return out, nil
}

func sendOnly(ctx context.Context, c *Client, subProto uint8, action string, req any) error {
	if err := c.validateRoute(); err != nil {
		return err
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	payload, err := transport.EncodeMessage(action, req)
	if err != nil {
		return err
	}
	return c.base.Send(c.header(subProto, len(payload)), payload)
}

func requireNonBlank(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func validatePage(offset, limit int) error {
	if offset < 0 {
		return errors.New("offset must be non-negative")
	}
	if limit < 0 {
		return errors.New("limit must be non-negative")
	}
	return nil
}
