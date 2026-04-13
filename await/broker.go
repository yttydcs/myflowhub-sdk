package await

// Context: This file provides shared client-side SDK behavior around broker.

import (
	"errors"
	"strings"
	"sync"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

var (
	ErrClosed       = errors.New("已关闭")
	ErrKeyInvalid   = errors.New("等待 key 无效")
	ErrKeyDuplicate = errors.New("重复注册等待 key")
)

// Key 用于匹配请求-响应：MsgID + SubProto + Action。
type Key struct {
	MsgID    uint32
	SubProto uint8
	Action   string
}

type msgSubKey struct {
	MsgID    uint32
	SubProto uint8
}

// Response 为 Awaiter 投递的响应结果（原始 header/payload + 已解析的 message）。
type Response struct {
	Header  core.IHeader
	Payload []byte
	Message transport.Message
}

// Result 表示一次等待的完成结果：要么收到 Response，要么 Err 非空。
type Result struct {
	Response Response
	Err      error
}

type Broker struct {
	mu       sync.Mutex
	closed   bool
	closeErr error

	waiters  map[Key]chan Result
	byMsgSub map[msgSubKey]int
}

func NewBroker() *Broker {
	return &Broker{
		waiters:  make(map[Key]chan Result),
		byMsgSub: make(map[msgSubKey]int),
	}
}

func normalizeKey(key Key) (Key, error) {
	key.Action = strings.TrimSpace(key.Action)
	if key.MsgID == 0 || key.Action == "" {
		return Key{}, ErrKeyInvalid
	}
	return key, nil
}

// HasMsgSub 用于快速判断：是否存在任意等待者，其 (msg_id, sub_proto) 与输入一致。
// 该方法用于 readLoop 快速路径：没有等待者时避免对 payload 做 JSON 解码。
func (b *Broker) HasMsgSub(msgID uint32, subProto uint8) bool {
	if msgID == 0 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.byMsgSub == nil {
		return false
	}
	return b.byMsgSub[msgSubKey{MsgID: msgID, SubProto: subProto}] > 0
}

// Register 注册一次等待。若重复注册同一 key，返回 ErrKeyDuplicate。
// 返回的 channel 为缓冲 1，Deliver 侧不应阻塞；完成后通道会被关闭。
func (b *Broker) Register(key Key) (<-chan Result, func(), error) {
	nk, err := normalizeKey(key)
	if err != nil {
		return nil, nil, err
	}

	b.mu.Lock()
	if b.closed {
		ce := b.closeErr
		if ce == nil {
			ce = ErrClosed
		}
		b.mu.Unlock()
		return nil, nil, ce
	}
	if b.waiters == nil {
		b.waiters = make(map[Key]chan Result)
	}
	if _, ok := b.waiters[nk]; ok {
		b.mu.Unlock()
		return nil, nil, ErrKeyDuplicate
	}
	ch := make(chan Result, 1)
	b.waiters[nk] = ch
	if b.byMsgSub == nil {
		b.byMsgSub = make(map[msgSubKey]int)
	}
	ms := msgSubKey{MsgID: nk.MsgID, SubProto: nk.SubProto}
	b.byMsgSub[ms]++
	b.mu.Unlock()

	cancel := func() { b.Cancel(nk) }
	return ch, cancel, nil
}

// Cancel 取消一次等待；若 key 不存在则返回 false。
func (b *Broker) Cancel(key Key) bool {
	nk, err := normalizeKey(key)
	if err != nil {
		return false
	}

	b.mu.Lock()
	ch, ok := b.waiters[nk]
	if ok {
		delete(b.waiters, nk)
		ms := msgSubKey{MsgID: nk.MsgID, SubProto: nk.SubProto}
		if b.byMsgSub != nil {
			if n := b.byMsgSub[ms] - 1; n <= 0 {
				delete(b.byMsgSub, ms)
			} else {
				b.byMsgSub[ms] = n
			}
		}
	}
	b.mu.Unlock()

	if ok {
		close(ch)
	}
	return ok
}

// Deliver 投递响应；若没有等待者则返回 false。
func (b *Broker) Deliver(key Key, resp Response) bool {
	nk, err := normalizeKey(key)
	if err != nil {
		return false
	}

	b.mu.Lock()
	ch, ok := b.waiters[nk]
	if ok {
		delete(b.waiters, nk)
		ms := msgSubKey{MsgID: nk.MsgID, SubProto: nk.SubProto}
		if b.byMsgSub != nil {
			if n := b.byMsgSub[ms] - 1; n <= 0 {
				delete(b.byMsgSub, ms)
			} else {
				b.byMsgSub[ms] = n
			}
		}
	}
	b.mu.Unlock()
	if !ok {
		return false
	}

	ch <- Result{Response: resp}
	close(ch)
	return true
}

// Close 关闭 Broker：唤醒并清理全部等待者，避免泄漏。
// err 为 nil 时使用 ErrClosed。
func (b *Broker) Close(err error) {
	if err == nil {
		err = ErrClosed
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.closeErr = err

	var chans []chan Result
	for _, ch := range b.waiters {
		chans = append(chans, ch)
	}
	b.waiters = nil
	b.byMsgSub = nil
	b.mu.Unlock()

	for _, ch := range chans {
		ch <- Result{Err: err}
		close(ch)
	}
}

// Reopen clears closed state so the broker can be reused after a reconnect.
// It does not recreate any waiter; callers should only use it when no active waiters are expected.
func (b *Broker) Reopen() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = false
	b.closeErr = nil
	if b.waiters == nil {
		b.waiters = make(map[Key]chan Result)
	}
	if b.byMsgSub == nil {
		b.byMsgSub = make(map[msgSubKey]int)
	}
}
