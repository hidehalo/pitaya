package world

import (
	"context"
	"errors"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/storage"
	"github.com/topfreegames/pitaya/v2/logger/interfaces"
	"github.com/topfreegames/pitaya/v2/serialize"
	"github.com/topfreegames/pitaya/v2/serialize/jsonpb"
)

var s serialize.Serializer = jsonpb.NewSerializer()
var cg map[string]bool = make(map[string]bool)

type ObjectMgr struct {
	Ctx       context.Context
	Id        string
	ScopeName string
	Redis     *redis.Client
	StreamMgr storage.StreamManager
	Logger    interfaces.Logger
}

func (w *ObjectMgr) Get() []*worldProto.EntityStatus {
	memRes := w.Redis.SMembers(w.Ctx, w.ScopeName+":object:set")
	if memRes.Err() != nil {
		w.Logger.Warnf("Redis set members err: %s", memRes.Err().Error())
		return make([]*worldProto.EntityStatus, 0)
	}
	keys := make([]string, 0)
	for _, id := range memRes.Val() {
		keys = append(keys, w.ScopeName+":object:key:"+id)
	}
	pipeRes := make([]*redis.IntCmd, 0)
	pipe := w.Redis.Pipeline()
	for _, key := range keys {
		pipeRes = append(pipeRes, pipe.Exists(w.Ctx, key))
	}
	_, err := pipe.Exec(w.Ctx)
	if err != nil {
		w.Logger.Warnf("Redis key exists err: %s", err.Error())
		return make([]*worldProto.EntityStatus, 0)
	}
	activeKeys := make([]string, 0)
	inActiveIds := make([]string, 0)
	for idx, res := range pipeRes {
		if res.Val() > 0 {
			activeKeys = append(activeKeys, keys[idx])
		} else {
			inActiveIds = append(inActiveIds, memRes.Val()[idx])
		}
	}
	w.Logger.Debugf("Active entities=%v", activeKeys)

	// remove in-active keys
	if len(inActiveIds) > 0 {
		removeTxn := w.Redis.TxPipeline()
		removeKeys := make([]string, 0)
		for _, removeId := range inActiveIds {
			removeKeys = append(removeKeys, w.ScopeName+":object:laststatus:"+removeId)
			removeKeys = append(removeKeys, w.ScopeName+":object:status:"+removeId)
		}
		sRemRes := removeTxn.SRem(w.Ctx, w.ScopeName+":object:set", inActiveIds)
		delRes := removeTxn.Del(w.Ctx, removeKeys...)
		_, err := removeTxn.Exec(w.Ctx)
		if err != nil {
			w.Logger.Warnf("Redis GC txn err=%s", err.Error())
		} else {
			w.Logger.Infof("Redis GC has run, should remove %d keys, actually remove %d keys", len(inActiveIds)+len(removeKeys), sRemRes.Val()+delRes.Val())
		}
	}

	mGetRes := w.Redis.MGet(w.Ctx, activeKeys...)
	if mGetRes.Err() != nil {
		w.Logger.Warnf("Redis key mget err: %s", mGetRes.Err())
		return make([]*worldProto.EntityStatus, 0)
	}
	entitiesId := mGetRes.Val()
	esSlice := make([]*worldProto.EntityStatus, 0)
	for _, id := range entitiesId {
		if idStr, ok := id.(string); ok {
			esSlice = append(esSlice, &worldProto.EntityStatus{Id: idStr})
		} else {
			w.Logger.Warnf("%v status id can't convert to string", entitiesId)
		}
	}
	return esSlice
}

func (w *ObjectMgr) Add(entities ...*worldProto.EntityStatus) error {
	pipe := w.Redis.Pipeline()
	expire := 30 * time.Second
	eIds := make([]string, 0)
	for _, es := range entities {
		eIds = append(eIds, es.Id)
		pipe.Set(w.Ctx, w.ScopeName+":object:key:"+es.Id, es.Id, expire)
	}
	pipe.SAdd(w.Ctx, w.ScopeName+":object:set", eIds)
	_, err := pipe.Exec(w.Ctx)
	if err != nil {
		return err
	}
	return nil
}

func (w *ObjectMgr) Del(entities ...*worldProto.EntityStatus) error {
	return nil
}

func (w *ObjectMgr) PutStream(msg *worldProto.EntityStatus) error {
	statusBytes, err := s.Marshal(msg)
	if err != nil {
		return err
	}
	resultStream := w.StreamMgr.CreateStream(w.ScopeName + ":object:status:" + msg.Id)
	resultValues := make(map[string]interface{})
	resultValues["status"] = statusBytes
	redisXMsg := redis.XMessage{
		ID:     "*",
		Values: resultValues,
	}
	redisStreamMessage := &storage.RedisStreamMessage{XMessage: redisXMsg}
	_, err = resultStream.Add(w.Ctx, redisStreamMessage)
	if err != nil {
		return err
	}
	return nil
}

func (w *ObjectMgr) ReadStream(ids []string) ([]*worldProto.EntityStatus, error) {
	streams := make([]string, 0)
	for _, id := range ids {
		resultStreamId := w.ScopeName + ":object:status:" + id
		streams = append(streams, resultStreamId)
		if _, ok := cg[resultStreamId]; !ok {
			xcRes := w.Redis.XGroupCreate(w.Ctx, resultStreamId, "gameLoop", "0")
			if xcRes.Err() == nil {
				cg[resultStreamId] = true
				w.Logger.Infof("Redis Stream %s XGroupCreate success", resultStreamId)
			} else {
				w.Logger.Warnf("Redis Stream %s XGroupCreate error %s", resultStreamId, xcRes.Err().Error())
			}
		}
	}

	suffix := strings.Split(strings.Repeat(">", len(streams)), "")
	streams = append(streams, suffix...)
	unSyncStatus := make([]*worldProto.EntityStatus, 0)
	res := w.Redis.XReadGroup(w.Ctx, &redis.XReadGroupArgs{
		Group:    "gameLoop",
		Consumer: w.Id,
		Streams:  streams,
		Count:    120,
		Block:    -1,
		NoAck:    true,
	})
	if res.Err() != nil && res.Err() != redis.Nil {
		w.Logger.Warnf("Redis XReadGroup streams %v failed, error is %s", streams, res.Err().Error())
		return make([]*worldProto.EntityStatus, 0), nil
	}

	for _, xStm := range res.Val() {
		for _, xMsg := range xStm.Messages {
			statusBytes := xMsg.Values["status"]
			if statusBytes == nil {
				w.Logger.Error("Read statusBytes")
				continue
			}
			entityStatus := &worldProto.EntityStatus{}
			err := s.Unmarshal([]byte(statusBytes.(string)), entityStatus)
			if statusBytes == nil {
				w.Logger.Error("Unmarshal statusBytes", err)
				continue
			}
			unSyncStatus = append(unSyncStatus, entityStatus)
		}
	}

	return unSyncStatus, nil
}

func (w *ObjectMgr) PutLastStatus(msg *worldProto.EntityStatus) error {
	if msg == nil {
		return errors.New("nil entity")
	}
	statusBytes, err := s.Marshal(msg)
	if err != nil {
		return err
	}
	res := w.Redis.Set(w.Ctx, w.ScopeName+":object:laststatus:"+msg.GetId(), statusBytes, 30*time.Second)
	// res := w.Redis.Set(w.Ctx, w.ScopeName+":object:laststatus:"+msg.GetId()+":uid", statusBytes, 30*time.Second)
	return res.Err()
}

func (w *ObjectMgr) GetLastStatus() ([]*worldProto.EntityStatus, error) {
	keyPattern := w.ScopeName + ":object:laststatus:*"
	keys := w.Redis.Keys(w.Ctx, keyPattern)
	statusSlice := make([]*worldProto.EntityStatus, 0)
	if len(keys.Val()) > 0 {
		res := w.Redis.MGet(w.Ctx, keys.Val()...)
		w.Logger.Infof("mget result=%v, error=%v", res.Val(), res.Err())
		for _, rawData := range res.Val() {
			entityStatus := &worldProto.EntityStatus{}
			err := s.Unmarshal([]byte(rawData.(string)), entityStatus)
			if err != nil {
				w.Logger.Error("Unmarshal rawData error", err)
				return nil, err
			}
			statusSlice = append(statusSlice, entityStatus)
		}
	}
	return statusSlice, nil
}
