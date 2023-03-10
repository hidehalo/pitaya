package storage

import (
	"context"
	"fmt"
	"testing"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestStreamConsumeGroup(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // use default Addr
		Password: "",               // no password set
		DB:       0,                // use default DB
	})
	sm := NewRedisStreamManager(redisClient)
	stm := sm.CreateStream("test:stream")
	_, err := stm.Trim(context.Background(), 0)
	assert.NoError(t, err)
	grp := stm.CreateConsumerGroup(context.Background(), "defaultGroup")
	csm := grp.CreateConsumer(context.Background(), "defaultConsumer")
	for i := 0; i < 3; i++ {
		_, err := stm.Add(context.Background(), &RedisStreamMessage{
			XMessage: redis.XMessage{
				ID: "*",
				Values: map[string]interface{}{
					"data": "Hi",
				},
			},
		})
		assert.NoError(t, err)
	}
	msgs, err := csm.Consume(context.Background(), 100)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(msgs))
	pMsgs, err := csm.GetPending(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 3, len(pMsgs))

	ids := make([]string, 0)
	for _, msg := range msgs {
		ids = append(ids, msg.GetID())
		fmt.Println(msg.GetID())
	}
	ar, err := csm.ACK(context.Background(), ids...)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(ids)), ar)

	pMsgs, err = csm.GetPending(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(pMsgs))

	_, err = stm.Trim(context.Background(), 0)
	assert.NoError(t, err)
}
