package typed

import (
	"context"
	"errors"
	"strings"

	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
)

var ErrPublishNameRequired = errors.New("topicbus publish name is required")

type TopicBusClient struct {
	client *Client
}

func (t *TopicBusClient) Subscribe(ctx context.Context, req topicbus.SubscribeReq) (topicbus.Resp, error) {
	return sendAndDecode[topicbus.Resp](ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionSubscribe, req, topicbus.ActionSubscribeResp)
}

func (t *TopicBusClient) SubscribeBatch(ctx context.Context, req topicbus.SubscribeBatchReq) (topicbus.Resp, error) {
	return sendAndDecode[topicbus.Resp](ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionSubscribeBatch, req, topicbus.ActionSubscribeBatchResp)
}

func (t *TopicBusClient) Unsubscribe(ctx context.Context, req topicbus.SubscribeReq) (topicbus.Resp, error) {
	return sendAndDecode[topicbus.Resp](ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionUnsubscribe, req, topicbus.ActionUnsubscribeResp)
}

func (t *TopicBusClient) UnsubscribeBatch(ctx context.Context, req topicbus.SubscribeBatchReq) (topicbus.Resp, error) {
	return sendAndDecode[topicbus.Resp](ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionUnsubscribeBatch, req, topicbus.ActionUnsubscribeBatchResp)
}

func (t *TopicBusClient) ListSubs(ctx context.Context) (topicbus.ListResp, error) {
	return sendAndDecode[topicbus.ListResp](ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionListSubs, struct{}{}, topicbus.ActionListSubsResp)
}

func (t *TopicBusClient) Publish(ctx context.Context, req topicbus.PublishReq) error {
	if strings.TrimSpace(req.Name) == "" {
		return ErrPublishNameRequired
	}
	return sendOnly(ctx, t.client, topicbus.SubProtoTopicBus, topicbus.ActionPublish, req)
}
