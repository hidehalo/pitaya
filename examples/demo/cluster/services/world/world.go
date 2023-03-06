package world

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pbProto "github.com/golang/protobuf/proto"
	redis "github.com/redis/go-redis/v9"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/storage"
	"github.com/topfreegames/pitaya/v2/examples/demo/protos"
	"github.com/topfreegames/pitaya/v2/serialize/jsonpb"
	"github.com/topfreegames/pitaya/v2/timer"
)

const (
	NewPlayer    string = "NewPlayer"
	PlayerQuit   string = "PlayerQuit"
	SyncAction   string = "SyncAction"
	GroupMembers string = "GroupMembers"
)

const WorldRoom string = "world"

const EntityStoreKey = "world:sync:entity:set"

type World struct {
	component.Base
	app       pitaya.Pitaya
	ticker    *timer.Timer
	streamMgr storage.StreamManager
	redis     *redis.Client
}

func NewWorld(app pitaya.Pitaya, streamMgr storage.StreamManager, redis *redis.Client) *World {
	return &World{
		app:       app,
		streamMgr: streamMgr,
		redis:     redis,
	}
}

func (w *World) Init() {
	w.app.GroupCreate(context.Background(), WorldRoom)
	w.syncLoop(context.Background())
	// tick := uint64(0)
	// logger := pitaya.GetDefaultLoggerFromCtx(context.Background())
	// s := jsonpb.NewSerializer()
	// w.ticker = pitaya.NewTimer(33*time.Millisecond, func() {
	// 	tick++
	// 	memberCount, _ := w.app.GroupCountMembers(context.Background(), WorldRoom)
	// 	if memberCount > 0 {
	// 		frameTick := &worldProto.FrameTick{
	// 			Tick: tick,
	// 			Mts:  float64(time.Now().UnixMicro()) / 1000,
	// 		}
	// 		res := w.redis.SMembers(context.Background(), "world:sync:entity")
	// 		if res.Err() != nil {
	// 			logger.Error(res.Err())
	// 		} else {
	// 			for _, entityId := range res.Val() {
	// 				resultStreamId := fmt.Sprintf("%s:%s:%s", w.app.GetServer().Type, "simulator:result", entityId)
	// 				resultStream := w.streamMgr.CreateStream(resultStreamId)
	// 				consumerGroup := resultStream.CreateConsumerGroup(context.Background(), "gameLoop")
	// 				consumer := consumerGroup.CreateConsumer(context.Background(), w.app.GetServerID())
	// 				streamMsgs, err := consumer.Consume(context.Background(), 120)
	// 				if err != nil {
	// 					logger.Error(err)
	// 				} else {
	// 					for _, xMsg := range streamMsgs {
	// 						statusBytes := xMsg.GetValue("status", nil)
	// 						if statusBytes == nil {
	// 							logger.Error(err)
	// 							continue
	// 						}
	// 						entityStatus := &worldProto.EntityStatus{}
	// 						err := s.Unmarshal(statusBytes.([]byte), entityStatus)
	// 						if statusBytes == nil {
	// 							logger.Error(err)
	// 							continue
	// 						}
	// 						frameTick.Status = append(frameTick.Status, entityStatus)
	// 					}
	// 				}
	// 			}
	// 		}

	// 		w.app.GroupBroadcast(context.Background(), "connector", WorldRoom, "FrameTick", frameTick)
	// 	}
	// })
}

func (w *World) AfterInit() {

}

func (w *World) BeforeShutdown() {
}

func (w *World) Shutdown() {
	w.app.GroupDelete(context.Background(), WorldRoom)
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

// Join world
func (w *World) Join(ctx context.Context) (*worldProto.Response, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	s := w.app.GetSessionFromCtx(ctx)
	fakeUID := s.ID()
	s.Bind(ctx, strconv.Itoa(int(fakeUID))) // binding session uid
	err := w.app.GroupAddMember(ctx, WorldRoom, s.UID())
	if err != nil {
		logger.Error("Failed to join world")
		logger.Error(err)
		return nil, err
	}
	// onclose callbacks are not allowed on backend servers
	// err = s.OnClose(func() {
	// 	w.app.GroupRemoveMember(ctx, WorldRoom, s.UID())
	// })
	// if err != nil {
	// 	return nil, err
	// }
	members, err := w.app.GroupMembers(ctx, WorldRoom)
	if err != nil {
		logger.Error("Failed to get members")
		logger.Error(err)
		return nil, err
	}
	allMemMsg := protos.AllMembers{Members: members}
	err = s.Push("GroupMembers", &allMemMsg)
	err = w.app.GroupBroadcast(ctx, "connector", WorldRoom, NewPlayer, &worldProto.PlayerJoin{Uuid: s.UID()})
	if err != nil {
		logger.Error("Failed to broadcast NewPlayer")
		logger.Error(err)
		return nil, err
	}
	return &worldProto.Response{Code: 200, Message: "ok"}, nil
}

func (w *World) Leave(ctx context.Context) (*worldProto.Response, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	s := w.app.GetSessionFromCtx(ctx)
	err := w.app.GroupRemoveMember(ctx, WorldRoom, s.UID())
	if err != nil {
		logger.Error("Failed to leave world")
		logger.Error(err)
		return nil, err
	}
	err = w.app.GroupBroadcast(ctx, "connector", WorldRoom, PlayerQuit, &worldProto.PlayerQuit{Uuid: s.UID()})
	if err != nil {
		logger.Error("Failed to broadcast PlayerQuit")
		logger.Error(err)
		return nil, err
	}
	return &worldProto.Response{Code: 200, Message: "ok"}, nil
}

func (w *World) SyncAction(ctx context.Context, msg *worldProto.SyncAction) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	serverTs := float64(time.Now().UnixNano()) / 1000
	msg.CreatedAt = &serverTs
	err := w.sendPushToOthers(ctx, SyncAction, msg)
	if err != nil {
		logger.Error("Error broadcasting SyncAction")
		logger.Error(err)
	}
}

func (w *World) sendPushToOthers(ctx context.Context, route string, msg pbProto.Message) error {
	uids, err := w.app.GroupMembers(ctx, WorldRoom)
	if err != nil {
		return err
	}
	session := w.app.GetSessionFromCtx(ctx)
	otherUids := make([]string, 0)
	for _, uid := range uids {
		if uid != session.UID() {
			otherUids = append(otherUids, uid)
		}
	}
	_, err = w.app.SendPushToUsers(route, msg, otherUids, "connector")
	if err != nil {
		return err
	}
	return nil
}

func (w *World) getSyncEntities() []*worldProto.EntityStatus {
	res := w.redis.SMembers(context.Background(), EntityStoreKey)
	entitiesId := res.Val()
	esSlice := make([]*worldProto.EntityStatus, 0)
	for _, id := range entitiesId {
		esSlice = append(esSlice, &worldProto.EntityStatus{Id: id})
	}
	return esSlice
}

func (w *World) addSyncEntity(entities ...*worldProto.EntityStatus) error {
	entitiesId := make([]string, 0)
	for _, es := range entities {
		entitiesId = append(entitiesId, es.Id)
	}
	res := w.redis.SAdd(context.Background(), EntityStoreKey, entitiesId)
	return res.Err()
}

func (w *World) delSyncEntity(entities ...*worldProto.EntityStatus) error {
	entitiesId := make([]string, 0)
	for _, es := range entities {
		entitiesId = append(entitiesId, es.Id)
	}
	res := w.redis.SRem(context.TODO(), EntityStoreKey, entitiesId)
	return res.Err()
}

func (w *World) syncLoop(ctx context.Context) {
	logger := pitaya.GetDefaultLoggerFromCtx(context.Background())
	s := jsonpb.NewSerializer()
	w.ticker = pitaya.NewTimer(time.Second/30, func() {
		memberCount, _ := w.app.GroupCountMembers(context.Background(), WorldRoom)
		if memberCount <= 0 {
			return
		}

		syncEntities := w.getSyncEntities()
		unSyncStatus := make([]*worldProto.EntityStatus, 0)

		for _, syncEntity := range syncEntities {
			resultStreamId := fmt.Sprintf("%s:%s:%s", w.app.GetServer().Type, "simulator:result", syncEntity.Id)
			resultStream := w.streamMgr.CreateStream(resultStreamId)
			consumerGroup := resultStream.CreateConsumerGroup(context.Background(), "gameLoop")
			consumer := consumerGroup.CreateConsumer(context.Background(), w.app.GetServerID())
			streamMsgs, err := consumer.Consume(context.Background(), 120)
			if err != nil {
				logger.Error(err)
			} else {
				for _, xMsg := range streamMsgs {
					statusBytes := xMsg.GetValue("status", nil)
					if statusBytes == nil {
						logger.Error(err)
						continue
					}
					entityStatus := &worldProto.EntityStatus{}
					err := s.Unmarshal(statusBytes.([]byte), entityStatus)
					if statusBytes == nil {
						logger.Error(err)
						continue
					}
					unSyncStatus = append(unSyncStatus, entityStatus)
				}
			}
		}

		w.app.GroupBroadcast(ctx, "connector", WorldRoom, "SyncStatus", unSyncStatus)
	})
}

func (w *World) rtt(ctx context.Context, rttAck *worldProto.RTTACK) float64 {
	mtsNow := float64(time.Now().UnixMilli()) / 1000
	rttMts := mtsNow - rttAck.StartMts
	return rttMts
}
