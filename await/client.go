package await

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-sdk/session"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

var (
	ErrClientNotInitialized = errors.New("client not initialized")
	ErrHeaderRequired       = errors.New("header is required")
	ErrActionRequired       = errors.New("action is required")
)

var msgSeq atomic.Uint32
var msgSeqInit sync.Once

func nextMsgID() uint32 {
	msgSeqInit.Do(func() {
		var seed [4]byte
		if _, err := rand.Read(seed[:]); err != nil {
			msgSeq.Store(uint32(time.Now().UnixNano()))
			return
		}
		msgSeq.Store(binary.BigEndian.Uint32(seed[:]))
	})

	v := msgSeq.Add(1)
	if v == 0 {
		v = msgSeq.Add(1)
	}
	return v
}

// Client 在 session.Session 之上提供 SendAndAwait：按 (MsgID + SubProto + Action) 等待响应。
// - 匹配成功的帧会被拦截投递，不再转交给 onUnmatched。
// - 未匹配帧不吞，会原样转交给 onUnmatched（例如 notify/broadcast/log 等）。
type Client struct {
	sess        *session.Session
	broker      *Broker
	onUnmatched func(core.IHeader, []byte)
	onError     func(error)
}

func NewClient(ctx context.Context, onUnmatched func(core.IHeader, []byte), onError func(error)) *Client {
	c := &Client{
		broker:      NewBroker(),
		onUnmatched: onUnmatched,
		onError:     onError,
	}
	c.sess = session.New(ctx, c.handleFrame, c.handleError)
	return c
}

func (c *Client) Connect(addr string) error {
	if c == nil || c.sess == nil {
		return ErrClientNotInitialized
	}
	return c.sess.Connect(addr)
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.sess != nil {
		c.sess.Close()
	}
	if c.broker != nil {
		c.broker.Close(ErrClosed)
	}
}

func (c *Client) Send(hdr core.IHeader, payload []byte) error {
	if c == nil || c.sess == nil {
		return ErrClientNotInitialized
	}
	return c.sess.Send(hdr, payload)
}

// SendAndAwait 发送请求并等待匹配的响应（按 MsgID+SubProto+Action）。
// - 若 hdr.MsgID==0：SDK 自动生成非 0 的 MsgID，并写回 hdr。
// - expectAction 用于匹配响应 payload 的 message.action。
func (c *Client) SendAndAwait(ctx context.Context, hdr core.IHeader, payload []byte, expectAction string) (Response, error) {
	if c == nil || c.sess == nil || c.broker == nil {
		return Response{}, ErrClientNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if hdr == nil {
		return Response{}, ErrHeaderRequired
	}
	expectAction = strings.TrimSpace(expectAction)
	if expectAction == "" {
		return Response{}, ErrActionRequired
	}

	if hdr.GetMsgID() == 0 {
		hdr.WithMsgID(nextMsgID())
	}

	k := Key{MsgID: hdr.GetMsgID(), SubProto: hdr.SubProto(), Action: expectAction}
	ch, cancel, err := c.broker.Register(k)
	if err != nil {
		return Response{}, err
	}
	defer cancel()

	if err := c.sess.Send(hdr, payload); err != nil {
		return Response{}, err
	}

	select {
	case r, ok := <-ch:
		if !ok {
			return Response{}, ErrClosed
		}
		if r.Err != nil {
			return Response{}, r.Err
		}
		return r.Response, nil
	case <-ctx.Done():
		// 尽量避免“响应与 ctx 同时到达”时误判为超时/取消：ctx 触发后再尝试读一次。
		select {
		case r, ok := <-ch:
			if ok && r.Err == nil {
				return r.Response, nil
			}
		default:
		}
		return Response{}, ctx.Err()
	}
}

func (c *Client) handleFrame(hdr core.IHeader, payload []byte) {
	if hdr == nil || c == nil || c.broker == nil {
		return
	}

	msgID := hdr.GetMsgID()
	sub := hdr.SubProto()
	if msgID == 0 || !c.broker.HasMsgSub(msgID, sub) {
		if c.onUnmatched != nil {
			c.onUnmatched(hdr, payload)
		}
		return
	}

	maj := hdr.Major()
	if maj != header.MajorOKResp && maj != header.MajorErrResp {
		if c.onUnmatched != nil {
			c.onUnmatched(hdr, payload)
		}
		return
	}

	msg, err := transport.DecodeMessage(payload)
	if err != nil {
		if c.onUnmatched != nil {
			c.onUnmatched(hdr, payload)
		}
		return
	}
	k := Key{MsgID: msgID, SubProto: sub, Action: msg.Action}
	resp := Response{Header: hdr, Payload: payload, Message: msg}
	if c.broker.Deliver(k, resp) {
		return
	}

	if c.onUnmatched != nil {
		c.onUnmatched(hdr, payload)
	}
}

func (c *Client) handleError(err error) {
	if err == nil || c == nil {
		return
	}
	if c.broker != nil {
		c.broker.Close(err)
	}
	if c.onError != nil {
		c.onError(err)
	}
}
