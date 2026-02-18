package await

import (
	"bufio"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	protocolfile "github.com/yttydcs/myflowhub-proto/protocol/file"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

func TestClient_SendAndAwait_AutoMsgID(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, _, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			return
		}

		respPayload, _ := transport.EncodeMessage("login_resp", map[string]any{"code": 1, "msg": "ok"})
		respHdr := (&header.HeaderTcp{}).
			WithMajor(header.MajorOKResp).
			WithSubProto(reqHdr.SubProto()).
			WithSourceID(reqHdr.TargetID()).
			WithTargetID(reqHdr.SourceID()).
			WithMsgID(reqHdr.GetMsgID()).
			WithPayloadLength(uint32(len(respPayload)))
		frame, _ := codec.Encode(respHdr, respPayload)
		_, _ = conn.Write(frame)
		<-serverDone
	}()

	unmatched := make(chan struct{}, 1)
	allFrames := make(chan struct{}, 8)
	client := NewClient(context.Background(),
		func(_hdr core.IHeader, _payload []byte) { unmatched <- struct{}{} },
		func(err error) { _ = err },
	)
	client.SetOnFrame(func(_hdr core.IHeader, _payload []byte) { allFrames <- struct{}{} })
	defer client.Close()

	if err := client.Connect(ln.Addr().String()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	reqPayload, _ := transport.EncodeMessage("login", map[string]any{"device_id": "d", "node_id": 1})
	reqHdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(2).
		WithSourceID(1).
		WithTargetID(0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.SendAndAwait(ctx, reqHdr, reqPayload, "login_resp")
	close(serverDone)
	if err != nil {
		t.Fatalf("send&await: %v", err)
	}
	if reqHdr.GetMsgID() == 0 {
		t.Fatalf("expected msg_id filled on request header")
	}
	if resp.Header.GetMsgID() != reqHdr.GetMsgID() {
		t.Fatalf("msg_id mismatch: resp=%d req=%d", resp.Header.GetMsgID(), reqHdr.GetMsgID())
	}
	if resp.Message.Action != "login_resp" {
		t.Fatalf("unexpected action: %q", resp.Message.Action)
	}

	select {
	case <-allFrames:
	default:
		t.Fatalf("expected onFrame callback")
	}
	select {
	case <-unmatched:
		t.Fatalf("unexpected unmatched callback")
	default:
	}
}

func TestClient_SendAndAwait_ActionMismatchTimesOutAndUnmatchedGetsFrame(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, _, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			return
		}

		respPayload, _ := transport.EncodeMessage("other_resp", map[string]any{"code": 1})
		respHdr := (&header.HeaderTcp{}).
			WithMajor(header.MajorOKResp).
			WithSubProto(reqHdr.SubProto()).
			WithMsgID(reqHdr.GetMsgID()).
			WithPayloadLength(uint32(len(respPayload)))
		frame, _ := codec.Encode(respHdr, respPayload)
		_, _ = conn.Write(frame)
		<-serverDone
	}()

	unmatched := make(chan struct{}, 1)
	client := NewClient(context.Background(),
		func(_hdr core.IHeader, _payload []byte) { unmatched <- struct{}{} },
		func(err error) { _ = err },
	)
	defer client.Close()

	if err := client.Connect(ln.Addr().String()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	reqPayload, _ := transport.EncodeMessage("login", map[string]any{"device_id": "d", "node_id": 1})
	reqHdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(2).
		WithSourceID(1).
		WithTargetID(0).
		WithMsgID(123)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = client.SendAndAwait(ctx, reqHdr, reqPayload, "login_resp")
	close(serverDone)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	select {
	case <-unmatched:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected unmatched callback")
	}
}

func TestClient_SendAndAwait_CancelBeforeResp_UnmatchedStillReceives(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, _, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			return
		}

		time.Sleep(120 * time.Millisecond)
		respPayload, _ := transport.EncodeMessage("login_resp", map[string]any{"code": 1})
		respHdr := (&header.HeaderTcp{}).
			WithMajor(header.MajorOKResp).
			WithSubProto(reqHdr.SubProto()).
			WithMsgID(reqHdr.GetMsgID()).
			WithPayloadLength(uint32(len(respPayload)))
		frame, _ := codec.Encode(respHdr, respPayload)
		_, _ = conn.Write(frame)
		time.Sleep(200 * time.Millisecond)
	}()

	unmatched := make(chan struct{}, 1)
	client := NewClient(context.Background(),
		func(_hdr core.IHeader, _payload []byte) { unmatched <- struct{}{} },
		func(err error) { _ = err },
	)
	defer client.Close()

	if err := client.Connect(ln.Addr().String()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	reqPayload, _ := transport.EncodeMessage("login", map[string]any{"device_id": "d", "node_id": 1})
	reqHdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(2).
		WithSourceID(1).
		WithTargetID(0).
		WithMsgID(555)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	_, err = client.SendAndAwait(ctx, reqHdr, reqPayload, "login_resp")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}

	select {
	case <-unmatched:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected unmatched callback after cancel")
	}
}

func TestClient_SendAndAwait_FileCtrlKindPrefixDecoded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, _, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			return
		}

		body, _ := transport.EncodeMessage(protocolfile.ActionReadResp, map[string]any{"code": 1, "msg": "ok"})
		respPayload := append([]byte{protocolfile.KindCtrl}, body...)
		respHdr := (&header.HeaderTcp{}).
			WithMajor(header.MajorOKResp).
			WithSubProto(reqHdr.SubProto()).
			WithSourceID(reqHdr.TargetID()).
			WithTargetID(reqHdr.SourceID()).
			WithMsgID(reqHdr.GetMsgID()).
			WithPayloadLength(uint32(len(respPayload)))
		frame, _ := codec.Encode(respHdr, respPayload)
		_, _ = conn.Write(frame)
		<-serverDone
	}()

	unmatched := make(chan struct{}, 1)
	allFrames := make(chan struct{}, 8)
	client := NewClient(context.Background(),
		func(_hdr core.IHeader, _payload []byte) { unmatched <- struct{}{} },
		func(err error) { _ = err },
	)
	client.SetOnFrame(func(_hdr core.IHeader, _payload []byte) { allFrames <- struct{}{} })
	defer client.Close()

	if err := client.Connect(ln.Addr().String()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	reqBody, _ := transport.EncodeMessage(protocolfile.ActionRead, map[string]any{"op": protocolfile.OpList, "target": 1})
	reqPayload := append([]byte{protocolfile.KindCtrl}, reqBody...)
	reqHdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(protocolfile.SubProtoFile).
		WithSourceID(1).
		WithTargetID(0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := client.SendAndAwait(ctx, reqHdr, reqPayload, protocolfile.ActionReadResp)
	close(serverDone)
	if err != nil {
		t.Fatalf("send&await: %v", err)
	}
	if resp.Message.Action != protocolfile.ActionReadResp {
		t.Fatalf("unexpected action: %q", resp.Message.Action)
	}

	select {
	case <-allFrames:
	default:
		t.Fatalf("expected onFrame callback")
	}
	select {
	case <-unmatched:
		t.Fatalf("unexpected unmatched callback")
	default:
	}
}
