package session

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
)

func TestSessionSend_FillsDefaults(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := New(context.Background(), nil, nil)
	sess.mu.Lock()
	sess.pipe = clientConn
	sess.mu.Unlock()

	payload := []byte("hello")
	hdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(1).
		WithSourceID(100).
		WithTargetID(200).
		WithMsgID(1).
		WithPayloadLength(uint32(len(payload)))

	sendErr := make(chan error, 1)
	go func() { sendErr <- sess.Send(hdr, payload) }()

	decodedHdr, decodedPayload, err := (header.HeaderTcpCodec{}).Decode(bufio.NewReader(serverConn))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send: %v", err)
	}

	if decodedHdr.GetHopLimit() != header.DefaultHopLimit {
		t.Fatalf("hop_limit not filled: got=%d", decodedHdr.GetHopLimit())
	}
	if decodedHdr.GetTraceID() == 0 {
		t.Fatalf("trace_id not filled")
	}
	if string(decodedPayload) != string(payload) {
		t.Fatalf("payload mismatch: got=%q want=%q", decodedPayload, payload)
	}
}

func TestSessionReadLoop_DispatchesFrames(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	type frame struct {
		h core.IHeader
		p []byte
	}
	got := make(chan frame, 1)
	sess := New(context.Background(),
		func(h core.IHeader, p []byte) { got <- frame{h: h, p: p} },
		func(err error) { t.Fatalf("onError: %v", err) },
	)

	sess.mu.Lock()
	sess.pipe = clientConn
	sess.mu.Unlock()
	go sess.readLoop(clientConn)

	payload := []byte("ping")
	hdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorMsg).
		WithSubProto(1).
		WithSourceID(1).
		WithTargetID(2).
		WithMsgID(9).
		WithPayloadLength(uint32(len(payload)))
	frameBytes, err := (header.HeaderTcpCodec{}).Encode(hdr, payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	go func() { _, _ = serverConn.Write(frameBytes) }()

	select {
	case f := <-got:
		if f.h == nil {
			t.Fatalf("nil header")
		}
		if f.h.GetMsgID() != 9 {
			t.Fatalf("msg_id mismatch: got=%d", f.h.GetMsgID())
		}
		if string(f.p) != "ping" {
			t.Fatalf("payload mismatch: got=%q", f.p)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for frame")
	}

	sess.Close()
}

type shortWritePipe struct {
	buf   bytes.Buffer
	limit int
}

func (p *shortWritePipe) Read([]byte) (int, error) { return 0, io.EOF }
func (p *shortWritePipe) Write(b []byte) (int, error) {
	n := len(b)
	if p.limit > 0 && n > p.limit {
		n = p.limit
	}
	if n > 0 {
		_, _ = p.buf.Write(b[:n])
	}
	return n, nil
}
func (p *shortWritePipe) Close() error { return nil }

func TestSessionSend_HandlesShortWritePipe(t *testing.T) {
	pipe := &shortWritePipe{limit: 3}
	sess := New(context.Background(), nil, nil)
	sess.mu.Lock()
	sess.pipe = pipe
	sess.mu.Unlock()

	payload := []byte("register-payload")
	hdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(2).
		WithSourceID(1).
		WithTargetID(2).
		WithMsgID(99).
		WithTraceID(77)

	if err := sess.Send(hdr, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	gotHdr, gotPayload, err := (header.HeaderTcpCodec{}).Decode(bytes.NewReader(pipe.buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if gotHdr.GetMsgID() != 99 {
		t.Fatalf("msg_id mismatch: got=%d want=99", gotHdr.GetMsgID())
	}
	if string(gotPayload) != string(payload) {
		t.Fatalf("payload mismatch: got=%q want=%q", gotPayload, payload)
	}
}

func TestSessionClose_DoesNotReportOnError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	errCh := make(chan error, 1)
	sess := New(context.Background(), nil, func(err error) {
		errCh <- err
	})

	sess.mu.Lock()
	sess.pipe = clientConn
	sess.mu.Unlock()
	go sess.readLoop(clientConn)

	sess.Close()

	select {
	case err := <-errCh:
		t.Fatalf("unexpected onError during close: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
}
