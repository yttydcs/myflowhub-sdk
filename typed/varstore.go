package typed

import (
	"context"
	"errors"
	"strings"

	"github.com/yttydcs/myflowhub-proto/protocol/varstore"
)

var (
	ErrVarNameInvalid      = errors.New("varstore name must contain only letters, digits, or underscore")
	ErrVarValueRequired    = errors.New("varstore value is required")
	ErrVisibilityInvalid   = errors.New("varstore visibility must be public or private")
	ErrSubscriberInvalid   = errors.New("varstore subscriber must be 0 or source_id")
	ErrSubscribeOwnerEmpty = errors.New("varstore subscribe owner is required")
)

type VarStoreClient struct {
	client *Client
}

func (v *VarStoreClient) Set(ctx context.Context, req varstore.SetReq) (varstore.VarResp, error) {
	if err := validateVarName(req.Name); err != nil {
		return varstore.VarResp{}, err
	}
	if strings.TrimSpace(req.Value) == "" {
		return varstore.VarResp{}, ErrVarValueRequired
	}
	if req.Visibility == "" {
		req.Visibility = varstore.VisibilityPublic
	}
	if err := validateVisibility(req.Visibility); err != nil {
		return varstore.VarResp{}, err
	}
	return sendAndDecode[varstore.VarResp](ctx, v.client, varstore.SubProtoVarStore, varstore.ActionSet, req, varstore.ActionSetResp)
}

func (v *VarStoreClient) Get(ctx context.Context, req varstore.GetReq) (varstore.VarResp, error) {
	if err := validateVarName(req.Name); err != nil {
		return varstore.VarResp{}, err
	}
	return sendAndDecode[varstore.VarResp](ctx, v.client, varstore.SubProtoVarStore, varstore.ActionGet, req, varstore.ActionGetResp)
}

func (v *VarStoreClient) List(ctx context.Context, req varstore.ListReq) (varstore.VarResp, error) {
	return sendAndDecode[varstore.VarResp](ctx, v.client, varstore.SubProtoVarStore, varstore.ActionList, req, varstore.ActionListResp)
}

func (v *VarStoreClient) Revoke(ctx context.Context, req varstore.GetReq) (varstore.VarResp, error) {
	if err := validateVarName(req.Name); err != nil {
		return varstore.VarResp{}, err
	}
	return sendAndDecode[varstore.VarResp](ctx, v.client, varstore.SubProtoVarStore, varstore.ActionRevoke, req, varstore.ActionRevokeResp)
}

func (v *VarStoreClient) Subscribe(ctx context.Context, req varstore.SubscribeReq) (varstore.VarResp, error) {
	if err := v.validateSubscribe(req); err != nil {
		return varstore.VarResp{}, err
	}
	return sendAndDecode[varstore.VarResp](ctx, v.client, varstore.SubProtoVarStore, varstore.ActionSubscribe, req, varstore.ActionSubscribeResp)
}

func (v *VarStoreClient) Unsubscribe(ctx context.Context, req varstore.SubscribeReq) error {
	if err := v.validateSubscribe(req); err != nil {
		return err
	}
	return sendOnly(ctx, v.client, varstore.SubProtoVarStore, varstore.ActionUnsubscribe, req)
}

func (v *VarStoreClient) validateSubscribe(req varstore.SubscribeReq) error {
	if err := v.client.validateRoute(); err != nil {
		return err
	}
	if err := validateVarName(req.Name); err != nil {
		return err
	}
	if req.Owner == 0 {
		return ErrSubscribeOwnerEmpty
	}
	if req.Subscriber != 0 && req.Subscriber != v.client.opts.SourceID {
		return ErrSubscriberInvalid
	}
	return nil
}

func validateVarName(name string) error {
	if name == "" {
		return ErrVarNameInvalid
	}
	for _, r := range name {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return ErrVarNameInvalid
	}
	return nil
}

func validateVisibility(visibility string) error {
	switch visibility {
	case varstore.VisibilityPublic, varstore.VisibilityPrivate:
		return nil
	default:
		return ErrVisibilityInvalid
	}
}
