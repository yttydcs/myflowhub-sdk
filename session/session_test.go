package session

import (
	"bufio"
	"context"
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
	sess.conn = clientConn
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
	sess.conn = clientConn
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
