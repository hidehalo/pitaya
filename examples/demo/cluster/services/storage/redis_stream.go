package storage

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type RedisStreamMessage struct {
	redis.XMessage
}

func (m *RedisStreamMessage) GetID() string {
	return m.ID
}

func (m *RedisStreamMessage) GetValue(key string, defaultVal interface{}) interface{} {
	if v, ex := m.Values[key]; ex {
		return v
	}
	return defaultVal
}

func (m *RedisStreamMessage) GetValues() map[string]interface{} {
	return m.Values
}

type MessageSlice []Message

func createFromRedisStreamMessage(redisMsgs []redis.XMessage) MessageSlice {
	ms := make(MessageSlice, 0)
	for _, rawMsg := range redisMsgs {
		ms = append(ms, &RedisStreamMessage{
			XMessage: rawMsg,
		})
	}
	return ms
}

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

func (c *RedisConsumer) Consume(ctx context.Context, maxLen int64) ([]Message, error) {
	streams := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.GetGroup().GetName(),
		Consumer: c.GetName(),
		Block:    -1,
		Streams:  []string{c.GetGroup().GetStream().GetName(), ">"},
		Count:    maxLen,
		NoAck:    false,
	})
	if streams.Err() != nil && streams.Err() != redis.Nil {
		return nil, streams.Err()
	}
	// if len(streams.Val()) > 0 {
	// 	fmt.Println("consumer read stream message", len(streams.Val()[0].Messages))
	// } else {
	// 	fmt.Println("consume stream is not ex", c.GetGroup().GetStream().GetName())
	// }
	var messages MessageSlice
	if len(streams.Val()) > 0 {
		messages = createFromRedisStreamMessage(streams.Val()[0].Messages)
	}
	return messages, nil
}

func (c *RedisConsumer) GetPending(ctx context.Context) ([]Message, error) {
	streams := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.GetGroup().GetName(),
		Consumer: c.GetName(),
		Block:    -1,
		Streams:  []string{c.GetGroup().GetStream().GetName(), "0"},
	})
	if streams.Err() != nil && streams.Err() != redis.Nil {
		return nil, streams.Err()
	}
	var messages MessageSlice
	if len(streams.Val()) > 0 {
		messages = createFromRedisStreamMessage(streams.Val()[0].Messages)
	}
	return messages, nil
}

func (c *RedisConsumer) ACK(ctx context.Context, ids ...string) (int64, error) {
	ack := c.client.XAck(ctx, c.GetGroup().GetStream().GetName(), c.GetGroup().GetName(), ids...)
	return ack.Val(), ack.Err()
}

func (c *RedisConsumer) Claim(ctx context.Context, consumer string, nice int64, ids []string) ([]Message, error) {
	messages := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   c.GetGroup().GetStream().GetName(),
		Group:    c.GetName(),
		Consumer: consumer,
		MinIdle:  time.Duration(nice),
		Messages: ids,
	})
	if messages.Err() != nil && messages.Err() != redis.Nil {
		return nil, messages.Err()
	}
	var messageSlice MessageSlice
	messageSlice = createFromRedisStreamMessage(messages.Val())
	return messageSlice, nil
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

func (g *RedisConsumerGroup) CreateConsumer(_ context.Context, consumerName string) Consumer {
	redisConsumer := g.createLocalConsumer(consumerName)
	g.consumers = append(g.consumers, redisConsumer)
	return redisConsumer
}

func (g *RedisConsumerGroup) createLocalConsumer(consumerName string) Consumer {
	return &RedisConsumer{
		name:   consumerName,
		group:  g,
		client: g.client,
	}
}

func (g *RedisConsumerGroup) GetConsumers(ctx context.Context) ([]Consumer, error) {
	consumersInfo := g.client.XInfoConsumers(ctx, g.GetStream().GetName(), g.GetName())
	if consumersInfo.Err() != nil && consumersInfo.Err() != redis.Nil {
		return nil, consumersInfo.Err()
	}
	consumers := make([]Consumer, 0)
	for _, consumerInfo := range consumersInfo.Val() {
		consumers = append(consumers, g.createLocalConsumer(consumerInfo.Name))
	}
	return consumers, nil
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
	streamInfoRes := s.client.XInfoStream(context.Background(), s.GetName())
	makeStream := false
	// TODO: stream 不存在则要创建 stream
	if streamInfoRes.Err() != nil && streamInfoRes.Err().Error() == "ERR no such key" {
		makeStream = true
	} else if streamInfoRes.Err() != nil {
		panic("streamInfoRes Error: " + streamInfoRes.Err().Error())
	}

	remoteCreated := false
	if !makeStream {
		groupInfosRes := s.client.XInfoGroups(ctx, s.GetName())
		if groupInfosRes.Err() != nil {
			panic("groupInfosRes Error" + groupInfosRes.Err().Error())
		}
		for _, groupInfo := range groupInfosRes.Val() {
			if groupInfo.Name == groupName {
				remoteCreated = true
				break
			}
		}
	}

	if !remoteCreated {
		var res *redis.StatusCmd
		if makeStream {
			res = s.client.XGroupCreateMkStream(ctx, s.GetName(), groupName, "0")
		} else {
			res = s.client.XGroupCreate(ctx, s.GetName(), groupName, "0")
		}
		if res.Err() != nil {
			panic("Create stream consumer group error: " + res.Err().Error())
		}
	}
	return redisConsumerGroup
}

func (s *RedisStream) GetConsumerGroups(_ context.Context) []ConsumerGroup {
	return s.consumerGroups
}

func (s *RedisStream) Add(ctx context.Context, msg Message) (string, error) {
	res := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.GetName(),
		MaxLen: s.maxLen,
		ID:     msg.GetID(),
		Values: msg.GetValues(),
	})
	return res.Val(), res.Err()
}

func (s *RedisStream) Del(ctx context.Context, ids ...string) (int64, error) {
	res := s.client.XDel(ctx, s.GetName(), ids...)
	return res.Val(), res.Err()
}

func (s *RedisStream) Read(ctx context.Context, id string, keys ...string) ([]Message, error) {
	streams := s.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{s.GetName(), id},
		Block:   -1,
	})
	if streams.Err() != nil && streams.Err() != redis.Nil {
		return nil, streams.Err()
	}
	var messageSlice MessageSlice
	if len(streams.Val()) > 0 {
		messageSlice = createFromRedisStreamMessage(streams.Val()[0].Messages)
	}
	return messageSlice, nil
}

func (s *RedisStream) Trim(ctx context.Context, maxLen int64) (int64, error) {
	res := s.client.XTrimMaxLen(ctx, s.GetName(), maxLen)
	return res.Val(), res.Err()
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

func NewRedisStreamManager(client *redis.Client) *RedisStreamManager {
	return &RedisStreamManager{
		client:  client,
		streams: make([]Stream, 0),
	}
}
