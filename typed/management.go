package typed

import (
	"context"

	"github.com/yttydcs/myflowhub-proto/protocol/management"
)

type ManagementClient struct {
	client *Client
}

func (m *ManagementClient) NodeEcho(ctx context.Context, req management.NodeEchoReq) (management.NodeEchoResp, error) {
	return sendAndDecode[management.NodeEchoResp](ctx, m.client, management.SubProtoManagement, management.ActionNodeEcho, req, management.ActionNodeEchoResp)
}

func (m *ManagementClient) NodeInfo(ctx context.Context) (management.NodeInfoResp, error) {
	return sendAndDecode[management.NodeInfoResp](ctx, m.client, management.SubProtoManagement, management.ActionNodeInfo, management.NodeInfoReq{}, management.ActionNodeInfoResp)
}

func (m *ManagementClient) ListNodes(ctx context.Context) (management.ListNodesResp, error) {
	return sendAndDecode[management.ListNodesResp](ctx, m.client, management.SubProtoManagement, management.ActionListNodes, management.ListNodesReq{}, management.ActionListNodesResp)
}

func (m *ManagementClient) ListSubtree(ctx context.Context) (management.ListSubtreeResp, error) {
	return sendAndDecode[management.ListSubtreeResp](ctx, m.client, management.SubProtoManagement, management.ActionListSubtree, management.ListSubtreeReq{}, management.ActionListSubtreeResp)
}

func (m *ManagementClient) ConfigGet(ctx context.Context, req management.ConfigGetReq) (management.ConfigResp, error) {
	if err := requireNonBlank("key", req.Key); err != nil {
		return management.ConfigResp{}, err
	}
	return sendAndDecode[management.ConfigResp](ctx, m.client, management.SubProtoManagement, management.ActionConfigGet, req, management.ActionConfigGetResp)
}

func (m *ManagementClient) ConfigSet(ctx context.Context, req management.ConfigSetReq) (management.ConfigResp, error) {
	if err := requireNonBlank("key", req.Key); err != nil {
		return management.ConfigResp{}, err
	}
	return sendAndDecode[management.ConfigResp](ctx, m.client, management.SubProtoManagement, management.ActionConfigSet, req, management.ActionConfigSetResp)
}

func (m *ManagementClient) ConfigList(ctx context.Context) (management.ConfigListResp, error) {
	return sendAndDecode[management.ConfigListResp](ctx, m.client, management.SubProtoManagement, management.ActionConfigList, management.ConfigListReq{}, management.ActionConfigListResp)
}
