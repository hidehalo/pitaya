package storage

import "context"

type Consumer interface {
	GetName() string
	GetGroup() ConsumerGroup
	Consume(ctx context.Context, maxLen int64) ([]Message, error)
	GetPending(ctx context.Context) ([]Message, error)
	ACK(ctx context.Context, ids ...string) (int64, error)
	Claim(ctx context.Context, consumer string, nice int64, ids []string) ([]Message, error)
}

type ConsumerGroup interface {
	GetStream() Stream
	GetName() string
	CreateConsumer(ctx context.Context, consumerName string) Consumer
	GetConsumers(ctx context.Context) ([]Consumer, error)
}

type Message interface {
	GetID() string
	GetValue(key string, defaultVal interface{}) interface{}
	GetValues() map[string]interface{}
}

type Stream interface {
	GetName() string
	CreateConsumerGroup(ctx context.Context, groupName string) ConsumerGroup
	GetConsumerGroups(ctx context.Context) []ConsumerGroup
	Add(ctx context.Context, msg Message) (string, error)
	Del(ctx context.Context, ids ...string) (int64, error)
	Read(ctx context.Context, id string, keys ...string) ([]Message, error)
	Trim(ctx context.Context, maxLen int64) (int64, error)
}

type StreamManager interface {
	GetStreams() []Stream
	CreateStream(streamName string) Stream
}
