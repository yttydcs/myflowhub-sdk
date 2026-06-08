package typed

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-proto/protocol/auth"
	"github.com/yttydcs/myflowhub-sdk/await"
)

func TestAuthRegisterUsesBootstrapRouteAndKeyPair(t *testing.T) {
	keys, err := GenerateAuthKeyPair()
	if err != nil {
		t.Fatalf("GenerateAuthKeyPair: %v", err)
	}
	addr, reqCh, cleanup := startResponseServer(t, header.MajorOKResp, auth.ActionRegisterResp, auth.RespData{
		Code:     202,
		DeviceID: "device-1",
		Status:   "pending",
	})
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := tc.Auth().Register(ctx, AuthRegisterRequest{
		DeviceID:    " device-1 ",
		DisplayName: " node one ",
		KeyPair:     keys,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.Code != 202 || resp.Status != "pending" {
		t.Fatalf("unexpected register response: %+v", resp)
	}

	req := <-reqCh
	assertHeader(t, req.Header, auth.SubProtoAuth, 0, 0)
	assertNonZeroMsgID(t, req.Header)
	if req.Message.Action != auth.ActionRegister {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body auth.RegisterData
	decodeRequestData(t, req.Message, &body)
	if body.DeviceID != "device-1" {
		t.Fatalf("unexpected device_id: %q", body.DeviceID)
	}
	if body.DisplayName != "node one" {
		t.Fatalf("unexpected display_name: %q", body.DisplayName)
	}
	if body.PubKey != keys.PublicKeyDERBase64 || body.NodePub != keys.PublicKeyDERBase64 {
		t.Fatalf("unexpected pubkey/node_pub: %+v", body)
	}
	if _, err := ParseAuthPublicKeyBase64(body.PubKey); err != nil {
		t.Fatalf("invalid pubkey: %v", err)
	}
}

func TestAuthLoginUsesBootstrapRouteAndES256Signature(t *testing.T) {
	keys, err := GenerateAuthKeyPair()
	if err != nil {
		t.Fatalf("GenerateAuthKeyPair: %v", err)
	}
	addr, reqCh, cleanup := startResponseServer(t, header.MajorOKResp, auth.ActionLoginResp, auth.RespData{
		Code:     1,
		DeviceID: "device-1",
		NodeID:   7,
		Status:   "approved",
	})
	defer cleanup()

	base, tc := connectTypedClient(t, addr)
	defer base.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := tc.Auth().Login(ctx, AuthLoginRequest{
		DeviceID:    " device-1 ",
		NodeID:      7,
		DisplayName: " node one ",
		TS:          1700000000,
		Nonce:       " nonce-1 ",
		KeyPair:     keys,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.Code != 1 || resp.NodeID != 7 {
		t.Fatalf("unexpected login response: %+v", resp)
	}

	req := <-reqCh
	assertHeader(t, req.Header, auth.SubProtoAuth, 0, 0)
	assertNonZeroMsgID(t, req.Header)
	if req.Message.Action != auth.ActionLogin {
		t.Fatalf("unexpected action: %q", req.Message.Action)
	}
	var body auth.LoginData
	decodeRequestData(t, req.Message, &body)
	if body.DeviceID != "device-1" || body.NodeID != 7 {
		t.Fatalf("unexpected login identity: %+v", body)
	}
	if body.DisplayName != "node one" {
		t.Fatalf("unexpected display_name: %q", body.DisplayName)
	}
	if body.TS != 1700000000 || body.Nonce != "nonce-1" || body.Alg != AuthAlgES256 {
		t.Fatalf("unexpected signing fields: %+v", body)
	}

	pub, err := ParseAuthPublicKeyBase64(keys.PublicKeyDERBase64)
	if err != nil {
		t.Fatalf("ParseAuthPublicKeyBase64: %v", err)
	}
	if !verifyLoginSig(t, pub, body) {
		t.Fatalf("login signature did not verify")
	}
}

func TestAuthRegisterLoginValidation(t *testing.T) {
	base := awaitClientForValidation()
	tc := New(base, Options{SourceID: 10, TargetID: 1})
	badRoute := &Options{SourceID: 1}

	if _, err := tc.Auth().Register(context.Background(), AuthRegisterRequest{}); !errors.Is(err, ErrDeviceIDRequired) {
		t.Fatalf("expected device id error, got %v", err)
	}
	if _, err := tc.Auth().Register(context.Background(), AuthRegisterRequest{DeviceID: "d"}); !errors.Is(err, ErrPublicKeyRequired) {
		t.Fatalf("expected pubkey error, got %v", err)
	}
	if _, err := tc.Auth().Register(context.Background(), AuthRegisterRequest{DeviceID: "d", PubKey: "bad"}); err == nil {
		t.Fatalf("expected invalid pubkey error")
	}
	if _, err := tc.Auth().Register(context.Background(), AuthRegisterRequest{DeviceID: "d", PubKey: "bad", Route: badRoute}); !errors.Is(err, ErrAuthRouteInvalid) {
		t.Fatalf("expected route error, got %v", err)
	}
	if _, err := tc.Auth().Login(context.Background(), AuthLoginRequest{DeviceID: "d"}); !errors.Is(err, ErrNodeIDRequired) {
		t.Fatalf("expected node id error, got %v", err)
	}
	if _, err := tc.Auth().Login(context.Background(), AuthLoginRequest{DeviceID: "d", NodeID: 1}); !errors.Is(err, ErrPrivateKeyRequired) {
		t.Fatalf("expected private key error, got %v", err)
	}
	if _, err := tc.Auth().Login(context.Background(), AuthLoginRequest{DeviceID: "d", NodeID: 1, Sig: "sig", Nonce: "n"}); !errors.Is(err, ErrTimestampRequired) {
		t.Fatalf("expected timestamp error, got %v", err)
	}
	if _, err := tc.Auth().Login(context.Background(), AuthLoginRequest{DeviceID: "d", NodeID: 1, Sig: "sig", TS: 1}); !errors.Is(err, ErrNonceRequired) {
		t.Fatalf("expected nonce error, got %v", err)
	}
	if _, err := tc.Auth().Login(context.Background(), AuthLoginRequest{DeviceID: "d", NodeID: 1, Sig: "sig", TS: 1, Nonce: "n", Alg: "HS256"}); !errors.Is(err, ErrUnsupportedAuthAlg) {
		t.Fatalf("expected alg error, got %v", err)
	}

	if _, err := New(base, Options{}).Auth().GetPerms(context.Background(), auth.PermsQueryData{NodeID: 1}); !errors.Is(err, ErrSourceIDRequired) {
		t.Fatalf("auth admin route validation was weakened, got %v", err)
	}
}

func awaitClientForValidation() *await.Client {
	return await.NewClient(context.Background(), nil, nil)
}

func verifyLoginSig(t *testing.T, pub *ecdsa.PublicKey, body auth.LoginData) bool {
	t.Helper()
	rawSig, err := base64.StdEncoding.DecodeString(body.Sig)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	digest := sha256.Sum256(LoginSignBytes(body.DeviceID, body.NodeID, body.TS, body.Nonce))
	return ecdsa.VerifyASN1(pub, digest[:], rawSig)
}
