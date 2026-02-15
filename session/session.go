package session

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
)

var (
	ErrAlreadyConnected = errors.New("已经连接")
	ErrNotConnected     = errors.New("尚未连接")
)

var traceSeq atomic.Uint32
var traceSeqInit sync.Once

func nextTraceID() uint32 {
	traceSeqInit.Do(func() {
		var seed [4]byte
		if _, err := rand.Read(seed[:]); err != nil {
			traceSeq.Store(uint32(time.Now().UnixNano()))
			return
		}
		traceSeq.Store(binary.BigEndian.Uint32(seed[:]))
	})

	v := traceSeq.Add(1)
	if v == 0 {
		v = traceSeq.Add(1)
	}
	return v
}

type Session struct {
	mu      sync.Mutex
	conn    net.Conn
	codec   header.HeaderTcpCodec
	baseCtx context.Context
	ctx     context.Context
	cancel  context.CancelFunc
	onFrame func(core.IHeader, []byte)
	onError func(error)
}

func New(ctx context.Context, onFrame func(core.IHeader, []byte), onError func(error)) *Session {
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithCancel(ctx)
	return &Session{
		baseCtx: ctx,
		ctx:     cctx,
		cancel:  cancel,
		codec:   header.HeaderTcpCodec{},
		onFrame: onFrame,
		onError: onError,
	}
}

func (s *Session) Connect(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		return ErrAlreadyConnected
	}

	if s.ctx == nil || s.ctx.Err() != nil {
		base := s.baseCtx
		if base == nil {
			base = context.Background()
		}
		s.ctx, s.cancel = context.WithCancel(base)
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(s.ctx, "tcp", addr)
	if err != nil {
		return err
	}
	s.conn = conn

	// 注意：把 conn 作为参数传入，避免 readLoop 与 Close 之间对 s.conn 指针产生数据竞争。
	go s.readLoop(conn)
	return nil
}

func (s *Session) Close() {
	s.mu.Lock()
	conn := s.conn
	s.conn = nil
	cancel := s.cancel
	s.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if cancel != nil {
		cancel()
	}
}

func (s *Session) Send(hdr core.IHeader, payload []byte) error {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return ErrNotConnected
	}
	if hdr == nil {
		return errors.New("header is required")
	}

	if hdr.GetHopLimit() == 0 {
		hdr.WithHopLimit(header.DefaultHopLimit)
	}
	if hdr.GetTraceID() == 0 {
		hdr.WithTraceID(nextTraceID())
	}

	frame, err := s.codec.Encode(hdr, payload)
	if err != nil {
		return err
	}
	_, err = conn.Write(frame)
	return err
}

func (s *Session) readLoop(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		hdr, payload, err := s.codec.Decode(reader)
		if err != nil {
			// Close() 会 cancel ctx 并关闭连接；此时解码返回的错误属于“正常退出”，不应上报为 onError。
			if s.ctx != nil && s.ctx.Err() != nil {
				return
			}
			if s.onError != nil {
				s.onError(err)
			}
			return
		}
		if s.onFrame != nil {
			s.onFrame(hdr, payload)
		}
	}
}

func (s *Session) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return "Session(nil)"
	}
	return fmt.Sprintf("Session(%s -> %s)", s.conn.LocalAddr(), s.conn.RemoteAddr())
}
