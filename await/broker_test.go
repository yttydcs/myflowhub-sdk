package await

import (
	"errors"
	"testing"
	"time"

	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

func TestBroker_RegisterDuplicate(t *testing.T) {
	b := NewBroker()
	k := Key{MsgID: 1, SubProto: 2, Action: "login_resp"}

	ch1, cancel1, err := b.Register(k)
	if err != nil || ch1 == nil || cancel1 == nil {
		t.Fatalf("register1 unexpected: ch_nil=%v cancel_nil=%v err=%v", ch1 == nil, cancel1 == nil, err)
	}
	ch2, cancel2, err := b.Register(k)
	if !errors.Is(err, ErrKeyDuplicate) {
		t.Fatalf("expected ErrKeyDuplicate, got %v", err)
	}
	if ch2 != nil || cancel2 != nil {
		t.Fatalf("expected nil ch/cancel for duplicate")
	}

	cancel1()
	select {
	case _, ok := <-ch1:
		if ok {
			t.Fatalf("expected closed channel")
		}
	default:
		// cancel closes; read should not block but allow scheduling
		time.Sleep(10 * time.Millisecond)
		_, ok := <-ch1
		if ok {
			t.Fatalf("expected closed channel after cancel")
		}
	}
}

func TestBroker_DeliverAndHasMsgSub(t *testing.T) {
	b := NewBroker()
	k1 := Key{MsgID: 7, SubProto: 2, Action: "a"}
	k2 := Key{MsgID: 7, SubProto: 2, Action: "b"}

	ch1, cancel1, err := b.Register(k1)
	if err != nil {
		t.Fatalf("register1: %v", err)
	}
	_, _, err = b.Register(k2)
	if err != nil {
		t.Fatalf("register2: %v", err)
	}
	if !b.HasMsgSub(7, 2) {
		t.Fatalf("expected HasMsgSub true")
	}
	cancel1()
	if !b.HasMsgSub(7, 2) {
		t.Fatalf("expected HasMsgSub true after cancel one")
	}

	msgPayload, _ := transport.EncodeMessage("b", map[string]any{"code": 1})
	msg, _ := transport.DecodeMessage(msgPayload)
	resp := Response{
		Header:  (&header.HeaderTcp{}).WithMajor(header.MajorOKResp).WithSubProto(2).WithMsgID(7),
		Payload: msgPayload,
		Message: msg,
	}
	if !b.Deliver(k2, resp) {
		t.Fatalf("expected deliver ok")
	}
	if b.HasMsgSub(7, 2) {
		t.Fatalf("expected HasMsgSub false after all removed")
	}

	select {
	case r, ok := <-ch1:
		if ok || r.Err != nil || r.Response.Header != nil || len(r.Response.Payload) != 0 || r.Response.Message.Action != "" || r.Response.Message.Data != nil {
			t.Fatalf("expected ch1 closed only, got ok=%v r=%+v", ok, r)
		}
	default:
		// allow scheduling
		time.Sleep(10 * time.Millisecond)
		if _, ok := <-ch1; ok {
			t.Fatalf("expected closed")
		}
	}
}

func TestBroker_CloseWakesWaiters(t *testing.T) {
	b := NewBroker()
	k := Key{MsgID: 9, SubProto: 2, Action: "x"}
	ch, _, err := b.Register(k)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	closeErr := errors.New("boom")
	b.Close(closeErr)

	select {
	case r, ok := <-ch:
		if !ok {
			t.Fatalf("expected one result then close")
		}
		if !errors.Is(r.Err, closeErr) {
			t.Fatalf("expected closeErr, got %v", r.Err)
		}
		_, ok = <-ch
		if ok {
			t.Fatalf("expected channel closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}

	if _, _, err := b.Register(k); !errors.Is(err, closeErr) {
		t.Fatalf("expected register returns closeErr, got %v", err)
	}
}

func TestBroker_DeliverNoWaiter(t *testing.T) {
	b := NewBroker()
	msgPayload, _ := transport.EncodeMessage("x", map[string]any{"code": 1})
	msg, _ := transport.DecodeMessage(msgPayload)
	resp := Response{
		Header:  (&header.HeaderTcp{}).WithMajor(header.MajorOKResp).WithSubProto(2).WithMsgID(1),
		Payload: msgPayload,
		Message: msg,
	}
	if b.Deliver(Key{MsgID: 1, SubProto: 2, Action: "x"}, resp) {
		t.Fatalf("expected deliver false")
	}
}

func TestBroker_ReopenAfterClose(t *testing.T) {
	b := NewBroker()
	k := Key{MsgID: 10, SubProto: 2, Action: "login_resp"}

	ch, _, err := b.Register(k)
	if err != nil {
		t.Fatalf("register before close: %v", err)
	}
	b.Close(errors.New("network down"))

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("waiter should be closed on broker close")
	}

	if _, _, err := b.Register(k); err == nil {
		t.Fatalf("register should fail on closed broker")
	}

	b.Reopen()

	ch2, cancel2, err := b.Register(k)
	if err != nil {
		t.Fatalf("register after reopen: %v", err)
	}
	cancel2()
	select {
	case _, ok := <-ch2:
		if ok {
			t.Fatalf("channel should be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatalf("channel should close after cancel")
	}
}

func TestBroker_KeyValidation(t *testing.T) {
	b := NewBroker()
	if _, _, err := b.Register(Key{}); !errors.Is(err, ErrKeyInvalid) {
		t.Fatalf("expected ErrKeyInvalid, got %v", err)
	}
	if b.Cancel(Key{}) {
		t.Fatalf("expected cancel false")
	}
}
