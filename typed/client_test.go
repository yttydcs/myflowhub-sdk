package typed

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-proto/protocol/auth"
	"github.com/yttydcs/myflowhub-proto/protocol/management"
	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-proto/protocol/varstore"
	"github.com/yttydcs/myflowhub-sdk/await"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

type capturedRequest struct {
	Header  core.IHeader
	Message transport.Message
}

func TestManagementNodeEchoSendsHeaderAndDecodes(t *testing.T) {
	addr, reqCh, cleanup := startResponseServer(t, header.MajorOKResp, management.ActionNodeEchoResp, management.NodeEchoResp{
		Code: 1,
		Echo: "ping",
	})
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := tc.Management().NodeEcho(ctx, management.NodeEchoReq{Message: "ping"})
	if err != nil {
		t.Fatalf("node echo: %v", err)
	}
	if resp.Echo != "ping" {
		t.Fatalf("unexpected echo: %q", resp.Echo)
	}

	req := <-reqCh
	assertHeader(t, req.Header, management.SubProtoManagement, 10, 1)
	assertNonZeroMsgID(t, req.Header)
	if req.Message.Action != management.ActionNodeEcho {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body management.NodeEchoReq
	decodeRequestData(t, req.Message, &body)
	if body.Message != "ping" {
		t.Fatalf("unexpected message: %q", body.Message)
	}
}

func TestAuthListRolesUsesSpecResponseShape(t *testing.T) {
	addr, reqCh, cleanup := startResponseServer(t, header.MajorOKResp, auth.ActionListRolesResp, ListRolesResp{
		Code:  1,
		Total: 1,
		Roles: []auth.RolePermEntry{{NodeID: 42, Role: "admin", Perms: []string{"flow.read"}}},
	})
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := tc.Auth().ListRoles(ctx, auth.ListRolesReq{Role: "admin"})
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if resp.Total != 1 || len(resp.Roles) != 1 || resp.Roles[0].NodeID != 42 {
		t.Fatalf("unexpected roles response: %+v", resp)
	}

	req := <-reqCh
	assertHeader(t, req.Header, auth.SubProtoAuth, 10, 1)
	assertNonZeroMsgID(t, req.Header)
	if req.Message.Action != auth.ActionListRoles {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body auth.ListRolesReq
	decodeRequestData(t, req.Message, &body)
	if body.Role != "admin" {
		t.Fatalf("unexpected role filter: %q", body.Role)
	}
}

func TestVarStoreSetUsesMajorCmdResponseAndDefaultsVisibility(t *testing.T) {
	addr, reqCh, cleanup := startResponseServer(t, header.MajorCmd, varstore.ActionSetResp, varstore.VarResp{
		Code:       1,
		Name:       "sensor_1",
		Value:      "22.5",
		Owner:      10,
		Visibility: varstore.VisibilityPublic,
		Type:       "string",
	})
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := tc.VarStore().Set(ctx, varstore.SetReq{Name: "sensor_1", Value: "22.5", Owner: 10})
	if err != nil {
		t.Fatalf("varstore set: %v", err)
	}
	if resp.Value != "22.5" || resp.Visibility != varstore.VisibilityPublic {
		t.Fatalf("unexpected varstore response: %+v", resp)
	}

	req := <-reqCh
	assertHeader(t, req.Header, varstore.SubProtoVarStore, 10, 1)
	assertNonZeroMsgID(t, req.Header)
	if req.Message.Action != varstore.ActionSet {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body varstore.SetReq
	decodeRequestData(t, req.Message, &body)
	if body.Visibility != varstore.VisibilityPublic {
		t.Fatalf("expected default public visibility, got %q", body.Visibility)
	}
}

func TestTopicBusPublishSendOnlyPreservesTopic(t *testing.T) {
	addr, reqCh, cleanup := startReadOnlyServer(t)
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	rawTopic := " topic/raw "
	err := tc.TopicBus().Publish(context.Background(), topicbus.PublishReq{
		Topic:   rawTopic,
		Name:    "event",
		TS:      1730000000000,
		Payload: json.RawMessage(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	req := <-reqCh
	assertHeader(t, req.Header, topicbus.SubProtoTopicBus, 10, 1)
	if req.Message.Action != topicbus.ActionPublish {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body topicbus.PublishReq
	decodeRequestData(t, req.Message, &body)
	if body.Topic != rawTopic {
		t.Fatalf("topic was changed: %q", body.Topic)
	}
}

func TestTypedResponseRequiresData(t *testing.T) {
	addr, _, cleanup := startResponseServer(t, header.MajorOKResp, management.ActionNodeEchoResp, nil)
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := tc.Management().NodeEcho(ctx, management.NodeEchoReq{Message: "ping"})
	if !errors.Is(err, ErrResponseDataEmpty) {
		t.Fatalf("expected empty data error, got %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	base := await.NewClient(context.Background(), nil, nil)
	tc := New(base, Options{SourceID: 10, TargetID: 1})

	if _, err := New(nil, Options{SourceID: 10, TargetID: 1}).Management().NodeInfo(context.Background()); !errors.Is(err, ErrAwaitClientRequired) {
		t.Fatalf("expected await client error, got %v", err)
	}
	if _, err := New(base, Options{SourceID: 10}).Management().NodeInfo(context.Background()); !errors.Is(err, ErrTargetIDRequired) {
		t.Fatalf("expected target error, got %v", err)
	}
	if _, err := tc.Management().ConfigGet(context.Background(), management.ConfigGetReq{}); err == nil {
		t.Fatalf("expected config key validation error")
	}
	if _, err := tc.Auth().IssueRegisterPermit(context.Background(), auth.IssueRegisterPermitReq{DeviceID: "d"}); err == nil {
		t.Fatalf("expected role validation error")
	}
	if _, err := tc.VarStore().Set(context.Background(), varstore.SetReq{Name: "bad-name", Value: "v"}); !errors.Is(err, ErrVarNameInvalid) {
		t.Fatalf("expected var name error, got %v", err)
	}
	if _, err := tc.VarStore().Set(context.Background(), varstore.SetReq{Name: "ok", Value: "   "}); !errors.Is(err, ErrVarValueRequired) {
		t.Fatalf("expected var value error, got %v", err)
	}
	if _, err := tc.VarStore().Subscribe(context.Background(), varstore.SubscribeReq{Name: "ok", Owner: 10, Subscriber: 99}); !errors.Is(err, ErrSubscriberInvalid) {
		t.Fatalf("expected subscriber error, got %v", err)
	}
	if err := tc.TopicBus().Publish(context.Background(), topicbus.PublishReq{Topic: "t", Name: "   "}); !errors.Is(err, ErrPublishNameRequired) {
		t.Fatalf("expected publish name error, got %v", err)
	}
}

func connectTypedClient(t *testing.T, addr string) (*await.Client, *Client) {
	t.Helper()
	base := await.NewClient(context.Background(), nil, func(err error) { _ = err })
	if err := base.Connect(addr); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return base, New(base, Options{SourceID: 10, TargetID: 1})
}

func startResponseServer(t *testing.T, respMajor uint8, respAction string, respData any) (string, <-chan capturedRequest, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	reqCh := make(chan capturedRequest, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(reqCh)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, payload, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		msg, err := transport.DecodeMessage(payload)
		if err != nil {
			t.Errorf("decode message: %v", err)
			return
		}
		reqCh <- capturedRequest{Header: reqHdr, Message: msg}

		respPayload, err := transport.EncodeMessage(respAction, respData)
		if err != nil {
			t.Errorf("encode response: %v", err)
			return
		}
		respHdr := (&header.HeaderTcp{}).
			WithMajor(respMajor).
			WithSubProto(reqHdr.SubProto()).
			WithSourceID(reqHdr.TargetID()).
			WithTargetID(reqHdr.SourceID()).
			WithMsgID(reqHdr.GetMsgID()).
			WithPayloadLength(uint32(len(respPayload)))
		frame, err := codec.Encode(respHdr, respPayload)
		if err != nil {
			t.Errorf("encode frame: %v", err)
			return
		}
		if _, err := conn.Write(frame); err != nil {
			t.Errorf("write response: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}()
	return ln.Addr().String(), reqCh, func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("server timeout")
		}
	}
}

func startReadOnlyServer(t *testing.T) (string, <-chan capturedRequest, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	reqCh := make(chan capturedRequest, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(reqCh)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		codec := header.HeaderTcpCodec{}
		reqHdr, payload, err := codec.Decode(bufio.NewReader(conn))
		if err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		msg, err := transport.DecodeMessage(payload)
		if err != nil {
			t.Errorf("decode message: %v", err)
			return
		}
		reqCh <- capturedRequest{Header: reqHdr, Message: msg}
	}()
	return ln.Addr().String(), reqCh, func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("server timeout")
		}
	}
}

func assertHeader(t *testing.T, hdr core.IHeader, subProto uint8, sourceID, targetID uint32) {
	t.Helper()
	if hdr.Major() != header.MajorCmd {
		t.Fatalf("expected MajorCmd, got %d", hdr.Major())
	}
	if hdr.SubProto() != subProto {
		t.Fatalf("expected subproto %d, got %d", subProto, hdr.SubProto())
	}
	if hdr.SourceID() != sourceID || hdr.TargetID() != targetID {
		t.Fatalf("unexpected route source=%d target=%d", hdr.SourceID(), hdr.TargetID())
	}
}

func assertNonZeroMsgID(t *testing.T, hdr core.IHeader) {
	t.Helper()
	if hdr.GetMsgID() == 0 {
		t.Fatalf("expected non-zero msg_id")
	}
}

func decodeRequestData(t *testing.T, msg transport.Message, out any) {
	t.Helper()
	if err := json.Unmarshal(msg.Data, out); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}
