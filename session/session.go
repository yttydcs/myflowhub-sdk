package session

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-core/listener/quic_listener"
	"github.com/yttydcs/myflowhub-core/listener/rfcomm_listener"
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
	pipe    io.ReadWriteCloser
	local   string
	remote  string
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
	// Backward compatible: Connect() keeps "tcp host:port" behavior.
	return s.ConnectEndpoint(addr)
}

// ConnectEndpoint connects to an endpoint.
//
// Supported:
// - tcp: "127.0.0.1:9000" (legacy) or "tcp://127.0.0.1:9000"
// - rfcomm: "bt+rfcomm://AA:BB:CC:DD:EE:FF?uuid=...&channel=...&secure=true"
// - quic: "quic://host:port?server_name=...&alpn=myflowhub&pin_sha256=..."
func (s *Session) ConnectEndpoint(endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pipe != nil {
		return ErrAlreadyConnected
	}

	if s.ctx == nil || s.ctx.Err() != nil {
		base := s.baseCtx
		if base == nil {
			base = context.Background()
		}
		s.ctx, s.cancel = context.WithCancel(base)
	}

	pipe, local, remote, err := s.dialEndpointLocked(endpoint)
	if err != nil {
		return err
	}
	s.pipe = pipe
	s.local = local
	s.remote = remote

	// 注意：把 pipe 作为参数传入，避免 readLoop 与 Close 之间对 s.pipe 指针产生数据竞争。
	go s.readLoop(pipe)
	return nil
}

func (s *Session) Close() {
	s.mu.Lock()
	pipe := s.pipe
	s.pipe = nil
	s.local = ""
	s.remote = ""
	cancel := s.cancel
	s.mu.Unlock()

	// 先取消上下文，再关闭底层 pipe，避免 readLoop 在断开场景误上报 onError。
	if cancel != nil {
		cancel()
	}
	if pipe != nil {
		_ = pipe.Close()
	}
}

func (s *Session) Send(hdr core.IHeader, payload []byte) error {
	s.mu.Lock()
	pipe := s.pipe
	s.mu.Unlock()

	if pipe == nil {
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
	return core.WriteAll(pipe, frame)
}

func (s *Session) readLoop(pipe io.ReadWriteCloser) {
	reader := bufio.NewReader(pipe)
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
	if s.pipe == nil {
		return "Session(nil)"
	}
	local := s.local
	remote := s.remote
	if local == "" && remote == "" {
		return "Session(pipe)"
	}
	return fmt.Sprintf("Session(%s -> %s)", local, remote)
}

func (s *Session) dialEndpointLocked(endpoint string) (pipe io.ReadWriteCloser, local string, remote string, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, "", "", errors.New("endpoint is empty")
	}

	// Legacy: "host:port" implies TCP.
	if !strings.Contains(endpoint, "://") {
		return s.dialTCP(endpoint)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, "", "", err
	}
	switch strings.ToLower(strings.TrimSpace(u.Scheme)) {
	case "tcp":
		if strings.TrimSpace(u.Host) == "" {
			return nil, "", "", errors.New("tcp endpoint host is empty")
		}
		return s.dialTCP(u.Host)
	case rfcomm_listener.EndpointSchemeRFCOMM:
		// Use a short dial timeout to avoid UI stalls.
		cctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
		defer cancel()
		conn, err := rfcomm_listener.DialEndpoint(cctx, endpoint)
		if err != nil {
			return nil, "", "", err
		}
		pipe := conn.Pipe()
		if pipe == nil {
			_ = conn.Close()
			return nil, "", "", errors.New("rfcomm dial returned nil pipe")
		}
		localAddr := ""
		remoteAddr := ""
		if conn.LocalAddr() != nil {
			localAddr = conn.LocalAddr().String()
		}
		if conn.RemoteAddr() != nil {
			remoteAddr = conn.RemoteAddr().String()
		}
		return pipe, localAddr, remoteAddr, nil
	case quic_listener.EndpointSchemeQUIC:
		cctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
		defer cancel()
		conn, err := quic_listener.DialEndpoint(cctx, endpoint)
		if err != nil {
			return nil, "", "", err
		}
		pipe := conn.Pipe()
		if pipe == nil {
			_ = conn.Close()
			return nil, "", "", errors.New("quic dial returned nil pipe")
		}
		localAddr := ""
		remoteAddr := ""
		if conn.LocalAddr() != nil {
			localAddr = conn.LocalAddr().String()
		}
		if conn.RemoteAddr() != nil {
			remoteAddr = conn.RemoteAddr().String()
		}
		return pipe, localAddr, remoteAddr, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported endpoint scheme: %s", u.Scheme)
	}
}

func (s *Session) dialTCP(addr string) (pipe io.ReadWriteCloser, local string, remote string, err error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(s.ctx, "tcp", addr)
	if err != nil {
		return nil, "", "", err
	}
	local = ""
	remote = ""
	if conn.LocalAddr() != nil {
		local = conn.LocalAddr().String()
	}
	if conn.RemoteAddr() != nil {
		remote = conn.RemoteAddr().String()
	}
	return conn, local, remote, nil
}
