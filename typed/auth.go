package typed

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yttydcs/myflowhub-proto/protocol/auth"
)

type AuthClient struct {
	client *Client
}

var ErrNodeIDRequired = errors.New("node_id is required")

var (
	ErrAuthRouteInvalid    = errors.New("auth route override must use both source_id and target_id or neither")
	ErrAuthNodePubMismatch = errors.New("node_pub must match pubkey")
)

// ListRolesResp mirrors the auth.md list_roles response shape. Proto currently
// provides ListRolesReq and RolePermEntry but no dedicated response struct.
type ListRolesResp struct {
	Code  int                  `json:"code"`
	Msg   string               `json:"msg,omitempty"`
	Total int                  `json:"total"`
	Roles []auth.RolePermEntry `json:"roles,omitempty"`
}

// AuthRegisterRequest builds an auth/register request.
// When Route is nil, register uses the unauthenticated direct bootstrap route (source=0,target=0).
type AuthRegisterRequest struct {
	DeviceID      string
	RequestedRole string
	JoinPermit    string
	PubKey        string
	NodePub       string
	DisplayName   string
	TS            int64
	Nonce         string
	KeyPair       AuthKeyPair
	Route         *Options
}

// AuthLoginRequest builds and signs an auth/login request.
// When Route is nil, login uses the unauthenticated direct bootstrap route (source=0,target=0).
type AuthLoginRequest struct {
	DeviceID    string
	NodeID      uint32
	DisplayName string
	TS          int64
	Nonce       string
	Sig         string
	Alg         string
	KeyPair     AuthKeyPair
	NonceBytes  int
	Route       *Options
}

func (a *AuthClient) Register(ctx context.Context, req AuthRegisterRequest) (auth.RespData, error) {
	data, route, err := buildAuthRegisterData(req)
	if err != nil {
		return auth.RespData{}, err
	}
	return sendAndDecodeWithOptions[auth.RespData](ctx, a.client, route, auth.SubProtoAuth, auth.ActionRegister, data, auth.ActionRegisterResp)
}

func (a *AuthClient) Login(ctx context.Context, req AuthLoginRequest) (auth.RespData, error) {
	data, route, err := buildAuthLoginData(req)
	if err != nil {
		return auth.RespData{}, err
	}
	return sendAndDecodeWithOptions[auth.RespData](ctx, a.client, route, auth.SubProtoAuth, auth.ActionLogin, data, auth.ActionLoginResp)
}

func (a *AuthClient) GetPerms(ctx context.Context, req auth.PermsQueryData) (auth.RespData, error) {
	if req.NodeID == 0 {
		return auth.RespData{}, ErrNodeIDRequired
	}
	return sendAndDecode[auth.RespData](ctx, a.client, auth.SubProtoAuth, auth.ActionGetPerms, req, auth.ActionGetPermsResp)
}

func (a *AuthClient) ListRoles(ctx context.Context, req auth.ListRolesReq) (ListRolesResp, error) {
	if err := validatePage(req.Offset, req.Limit); err != nil {
		return ListRolesResp{}, err
	}
	return sendAndDecode[ListRolesResp](ctx, a.client, auth.SubProtoAuth, auth.ActionListRoles, req, auth.ActionListRolesResp)
}

func (a *AuthClient) ListPendingRegisters(ctx context.Context, req auth.ListPendingRegistersReq) (auth.ListPendingRegistersResp, error) {
	if err := validatePage(req.Offset, req.Limit); err != nil {
		return auth.ListPendingRegistersResp{}, err
	}
	return sendAndDecode[auth.ListPendingRegistersResp](ctx, a.client, auth.SubProtoAuth, auth.ActionListPendingRegisters, req, auth.ActionListPendingRegistersResp)
}

func (a *AuthClient) ListRegisterPermits(ctx context.Context, req auth.ListRegisterPermitsReq) (auth.ListRegisterPermitsResp, error) {
	if err := validatePage(req.Offset, req.Limit); err != nil {
		return auth.ListRegisterPermitsResp{}, err
	}
	return sendAndDecode[auth.ListRegisterPermitsResp](ctx, a.client, auth.SubProtoAuth, auth.ActionListRegisterPermits, req, auth.ActionListRegisterPermitsResp)
}

func (a *AuthClient) ApproveRegister(ctx context.Context, req auth.ApproveRegisterReq) (auth.ApproveRegisterResp, error) {
	if err := requireNonBlank("request_id", req.RequestID); err != nil {
		return auth.ApproveRegisterResp{}, err
	}
	return sendAndDecode[auth.ApproveRegisterResp](ctx, a.client, auth.SubProtoAuth, auth.ActionApproveRegister, req, auth.ActionApproveRegisterResp)
}

func (a *AuthClient) RejectRegister(ctx context.Context, req auth.RejectRegisterReq) (auth.RejectRegisterResp, error) {
	if err := requireNonBlank("request_id", req.RequestID); err != nil {
		return auth.RejectRegisterResp{}, err
	}
	return sendAndDecode[auth.RejectRegisterResp](ctx, a.client, auth.SubProtoAuth, auth.ActionRejectRegister, req, auth.ActionRejectRegisterResp)
}

func (a *AuthClient) IssueRegisterPermit(ctx context.Context, req auth.IssueRegisterPermitReq) (auth.IssueRegisterPermitResp, error) {
	if err := requireNonBlank("device_id", req.DeviceID); err != nil {
		return auth.IssueRegisterPermitResp{}, err
	}
	if err := requireNonBlank("role", req.Role); err != nil {
		return auth.IssueRegisterPermitResp{}, err
	}
	return sendAndDecode[auth.IssueRegisterPermitResp](ctx, a.client, auth.SubProtoAuth, auth.ActionIssueRegisterPermit, req, auth.ActionIssueRegisterPermitResp)
}

func (a *AuthClient) RevokeRegisterPermit(ctx context.Context, req auth.RevokeRegisterPermitReq) (auth.RevokeRegisterPermitResp, error) {
	if err := requireNonBlank("permit", req.Permit); err != nil {
		return auth.RevokeRegisterPermitResp{}, err
	}
	return sendAndDecode[auth.RevokeRegisterPermitResp](ctx, a.client, auth.SubProtoAuth, auth.ActionRevokeRegisterPermit, req, auth.ActionRevokeRegisterPermitResp)
}

func buildAuthRegisterData(req AuthRegisterRequest) (auth.RegisterData, Options, error) {
	route, err := authBootstrapRoute(req.Route)
	if err != nil {
		return auth.RegisterData{}, Options{}, err
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return auth.RegisterData{}, Options{}, ErrDeviceIDRequired
	}
	pubKey := strings.TrimSpace(req.PubKey)
	if pubKey == "" {
		pubKey, err = publicKeyFromPair(req.KeyPair)
		if err != nil {
			return auth.RegisterData{}, Options{}, err
		}
	} else if _, err := ParseAuthPublicKeyBase64(pubKey); err != nil {
		return auth.RegisterData{}, Options{}, err
	}
	nodePub := strings.TrimSpace(req.NodePub)
	if nodePub == "" {
		nodePub = pubKey
	} else if nodePub != pubKey {
		return auth.RegisterData{}, Options{}, ErrAuthNodePubMismatch
	}
	return auth.RegisterData{
		DeviceID:      deviceID,
		RequestedRole: strings.TrimSpace(req.RequestedRole),
		JoinPermit:    strings.TrimSpace(req.JoinPermit),
		PubKey:        pubKey,
		NodePub:       nodePub,
		DisplayName:   strings.TrimSpace(req.DisplayName),
		TS:            req.TS,
		Nonce:         strings.TrimSpace(req.Nonce),
	}, route, nil
}

func buildAuthLoginData(req AuthLoginRequest) (auth.LoginData, Options, error) {
	route, err := authBootstrapRoute(req.Route)
	if err != nil {
		return auth.LoginData{}, Options{}, err
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return auth.LoginData{}, Options{}, ErrDeviceIDRequired
	}
	if req.NodeID == 0 {
		return auth.LoginData{}, Options{}, ErrNodeIDRequired
	}
	alg, err := ensureES256(req.Alg)
	if err != nil {
		return auth.LoginData{}, Options{}, err
	}

	ts := req.TS
	nonce := strings.TrimSpace(req.Nonce)
	sig := strings.TrimSpace(req.Sig)
	if sig == "" {
		if ts == 0 {
			ts = time.Now().Unix()
		}
		if nonce == "" {
			nonce, err = GenerateAuthNonce(req.NonceBytes)
			if err != nil {
				return auth.LoginData{}, Options{}, err
			}
		}
		priv, err := privateKeyFromPair(req.KeyPair)
		if err != nil {
			return auth.LoginData{}, Options{}, err
		}
		sig, err = SignLogin(priv, deviceID, req.NodeID, ts, nonce)
		if err != nil {
			return auth.LoginData{}, Options{}, err
		}
	} else {
		if ts == 0 {
			return auth.LoginData{}, Options{}, ErrTimestampRequired
		}
		if nonce == "" {
			return auth.LoginData{}, Options{}, ErrNonceRequired
		}
	}

	return auth.LoginData{
		DeviceID:    deviceID,
		NodeID:      req.NodeID,
		DisplayName: strings.TrimSpace(req.DisplayName),
		TS:          ts,
		Nonce:       nonce,
		Sig:         sig,
		Alg:         alg,
	}, route, nil
}

func authBootstrapRoute(route *Options) (Options, error) {
	if route == nil {
		return Options{}, nil
	}
	if (route.SourceID == 0) != (route.TargetID == 0) {
		return Options{}, ErrAuthRouteInvalid
	}
	return *route, nil
}
