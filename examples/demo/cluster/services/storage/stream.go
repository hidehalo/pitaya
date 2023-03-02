package storage

import "context"

type Consumer interface {
	GetName() string
	GetGroup() ConsumerGroup
	Consume(ctx context.Context, maxLen int64)
	GetPending(ctx context.Context)
	ACK(ctx context.Context, ids ...string)
	Claim(ctx context.Context, consumer string, nice int64, ids []string)
}

type ConsumerGroup interface {
	GetStream() Stream
	GetName() string
	AddConsumer(ctx context.Context, consumerName string) Consumer
	GetConsumers(ctx context.Context) []Consumer
	GetPending(ctx context.Context)
}

type Stream interface {
	GetName() string
	CreateConsumerGroup(ctx context.Context, groupName string) ConsumerGroup
	GetConsumerGroups(ctx context.Context) []ConsumerGroup
	Add(ctx context.Context, id string, values interface{})
	Del(ctx context.Context, ids ...string)
	Trim(ctx context.Context, maxLen int64)
}

type StreamManager interface {
	GetStreams() []Stream
	CreateStream(streamName string) Stream
}
