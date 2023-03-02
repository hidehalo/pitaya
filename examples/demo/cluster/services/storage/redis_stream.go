package storage

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	Consumer
	name   string
	group  ConsumerGroup
	client *redis.Client
}

func (c *RedisConsumer) GetName() string {
	return c.name
}

func (c *RedisConsumer) GetGroup() ConsumerGroup {
	return c.group
}

func (c *RedisConsumer) Consume(ctx context.Context, maxLen int64) {
	c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.GetGroup().GetName(),
		Consumer: c.GetName(),
		Block:    -1,
		Streams:  []string{c.GetGroup().GetStream().GetName(), ">"},
		Count:    maxLen,
	})
}

func (c *RedisConsumer) GetPending(ctx context.Context) {
	c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.GetGroup().GetName(),
		Consumer: c.GetName(),
		Block:    -1,
		Streams:  []string{c.GetGroup().GetStream().GetName(), "0"},
	})
}

func (c *RedisConsumer) ACK(ctx context.Context, ids ...string) {
	c.client.XAck(ctx, c.GetGroup().GetStream().GetName(), c.GetName(), ids...)
}

func (c *RedisConsumer) Claim(ctx context.Context, consumer string, nice int64, ids []string) {
	c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   c.GetGroup().GetStream().GetName(),
		Group:    c.GetName(),
		Consumer: consumer,
		MinIdle:  time.Duration(nice),
		Messages: ids,
	})
}

type RedisConsumerGroup struct {
	stream    Stream
	name      string
	consumers []Consumer
	client    *redis.Client
}

func (g *RedisConsumerGroup) GetStream() Stream {
	return g.stream
}

func (g *RedisConsumerGroup) GetName() string {
	return g.name
}

func (g *RedisConsumerGroup) AddConsumer(_ context.Context, consumerName string) Consumer {
	redisConsumer := &RedisConsumer{
		name:   consumerName,
		group:  g,
		client: g.client,
	}
	g.consumers = append(g.consumers, redisConsumer)
	return redisConsumer
}

func (g *RedisConsumerGroup) GetConsumers(_ context.Context) []Consumer {
	return g.consumers
}

func (g *RedisConsumerGroup) GetPending(ctx context.Context) {
	g.client.XPending(ctx, g.GetStream().GetName(), g.GetName())
}

type RedisStream struct {
	name           string
	maxLen         int64
	client         *redis.Client
	consumerGroups []ConsumerGroup
}

func NewRedisStream(name string, maxLen int64, client *redis.Client) *RedisStream {
	return &RedisStream{
		name:   name,
		maxLen: maxLen,
		client: client,
	}
}

func (s *RedisStream) GetName() string {
	return s.name
}

func (s *RedisStream) CreateConsumerGroup(ctx context.Context, groupName string) ConsumerGroup {
	redisConsumerGroup := &RedisConsumerGroup{
		stream:    s,
		name:      groupName,
		consumers: make([]Consumer, 0),
		client:    s.client,
	}
	s.consumerGroups = append(s.consumerGroups, redisConsumerGroup)
	return redisConsumerGroup
}

func (s *RedisStream) GetConsumerGroups(_ context.Context) []ConsumerGroup {
	return s.consumerGroups
}

func (s *RedisStream) Add(ctx context.Context, id string, values interface{}) {
	s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.GetName(),
		MaxLen: s.maxLen,
		ID:     id + "-*",
		Values: values,
	})
}

func (s *RedisStream) Del(ctx context.Context, ids ...string) {
	s.client.XDel(ctx, s.GetName(), ids...)
}

func (s *RedisStream) Trim(ctx context.Context, maxLen int64) {
	s.client.XTrimMaxLen(ctx, s.GetName(), maxLen)
}

type RedisStreamManager struct {
	client  *redis.Client
	streams []Stream
}

func (m *RedisStreamManager) GetStreams() []Stream {
	return m.streams
}

func (m *RedisStreamManager) CreateStream(streamName string) Stream {
	redisStream := NewRedisStream(streamName, 1000, m.client)
	m.streams = append(m.streams, redisStream)
	return redisStream
}
