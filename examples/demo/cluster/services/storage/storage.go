package storage

import (
	"context"
)

type PubSub interface {
	Pub(ctx context.Context, channel string, message interface{}) error
	Sub(ctx context.Context, channel string) (interface{}, error)
}

type KeyType string

type KvStore interface {
	Put(ctx context.Context, key KeyType, tick uint64, value interface{}) (string, error)
	Read(ctx context.Context, key KeyType, tick uint64) (interface{}, error)
	Scan(ctx context.Context, key KeyType) (interface{}, error)
}

type Storage interface {
	PubSub() PubSub
	KvStore() KvStore
}
