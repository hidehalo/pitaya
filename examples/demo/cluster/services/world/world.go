package world

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/storage"
	"github.com/topfreegames/pitaya/v2/logger/interfaces"
	"github.com/topfreegames/pitaya/v2/serialize"
	"github.com/topfreegames/pitaya/v2/serialize/jsonpb"
)

const (
	NewPlayer    string = "NewPlayer"
	PlayerQuit   string = "PlayerQuit"
	SyncAction   string = "SyncAction"
	GroupMembers string = "GroupMembers"
)

const WorldRoom string = "world"

const EntityStoreKey = "world:sync:entity:set"

const FrontEndType = "room"

type World struct {
	component.Base
	app       pitaya.Pitaya
	ticker    *time.Ticker
	streamMgr storage.StreamManager
	redis     *redis.Client
	rttMap    map[string]time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	logger    interfaces.Logger
	cg        map[string]bool
	s         serialize.Serializer
	region    string
}

func NewWorld(app pitaya.Pitaya, streamMgr storage.StreamManager, redis *redis.Client) *World {
	ctx, cancel := context.WithCancel(context.Background())
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	return &World{
		app:       app,
		streamMgr: streamMgr,
		redis:     redis,
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		cg:        make(map[string]bool),
		s:         jsonpb.NewSerializer(),
		region:    fmt.Sprintf("%s:%s:%s", WorldRoom, app.GetServer().Type, app.GetServer().ID),
	}
}

func (w *World) getRegion() string {
	return w.region
}

func (w *World) Init() {
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		rkRes := w.redis.Keys(w.ctx, "room:*")
		w.redis.Del(w.ctx, rkRes.Val()...)
	}()

	go func() {
		defer wg.Done()
		wkRes := w.redis.Keys(w.ctx, "world:*")
		w.redis.Del(w.ctx, wkRes.Val()...)
	}()

	go func() {
		defer wg.Done()
		w.app.GroupCreate(w.ctx, WorldRoom)
	}()
	wg.Wait()

	go w.syncLoop(w.ctx)
}

func (w *World) AfterInit() {

}

func (w *World) BeforeShutdown() {
	w.app.GroupDelete(context.Background(), WorldRoom)
	if w.ticker != nil {
		w.ticker.Stop()
	}
	w.cancel()
}

func (w *World) Shutdown() {

}

// Join world
func (w *World) Join(ctx context.Context, initStatus *worldProto.EntityStatus) (*worldProto.Response, error) {
	logger := w.logger
	s := w.app.GetSessionFromCtx(ctx)
	fakeUID := s.ID()
	s.Bind(ctx, strconv.Itoa(int(fakeUID))) // binding session uid
	err := w.app.GroupAddMember(ctx, WorldRoom, s.UID())
	if err != nil {
		logger.Error("Failed to join world")
		logger.Error(err)
		return nil, err
	}
	s.SetData(map[string]interface{}{
		"group": WorldRoom,
	})
	// onclose callbacks are not allowed on backend servers
	s.OnClose(func() {
		w.app.GroupRemoveMember(ctx, WorldRoom, s.UID())
		s.SetData(map[string]interface{}{
			"group": "",
		})
	})
	// TODO: push to stream
	logger.Infof("Joins %s state = %d", initStatus.GetId(), initStatus.GetState())
	objMgr := ObjectMgr{
		Ctx:       ctx,
		Id:        w.app.GetServerID(),
		ScopeName: w.getRegion(),
		Redis:     w.redis,
		StreamMgr: w.streamMgr,
		Logger:    w.logger,
	}
	err = objMgr.Add(initStatus)
	if err != nil {
		return nil, err
	}
	err = objMgr.PutLastStatus(initStatus)
	if err != nil {
		return nil, err
	}
	statusSlice, err := objMgr.GetLastStatus()
	if err != nil {
		return nil, err
	}

	// statusBytes, err := w.s.Marshal(initStatus)
	// if err != nil {
	// 	logger.Error(err)
	// 	return nil, err
	// }
	// key := fmt.Sprintf("room:%s:laststatus:%s", w.app.GetServerID(), initStatus.GetId())
	// setRes := w.redis.Set(ctx, key, statusBytes, 30*time.Second)
	// w.logger.Infof("Set key %s result=%s", key, setRes.Val())
	// if setRes.Err() != nil {
	// 	return nil, setRes.Err()
	// }
	// keyPattern := fmt.Sprintf("room:%s:laststatus:*", w.app.GetServerID())
	// keys := w.redis.Keys(w.ctx, keyPattern)
	// w.logger.Infof("keys result=%v, error=%v", keys.Val(), keys.Err())
	// statusSlice := make([]*worldProto.EntityStatus, 0)
	// if len(keys.Val()) > 0 {
	// 	res := w.redis.MGet(w.ctx, keys.Val()...)
	// 	w.logger.Infof("mget result=%v, error=%v", res.Val(), res.Err())
	// 	for _, rawData := range res.Val() {
	// 		entityStatus := &worldProto.EntityStatus{}
	// 		err := w.s.Unmarshal([]byte(rawData.(string)), entityStatus)
	// 		if err != nil {
	// 			w.logger.Error("Unmarshal rawData error", err)
	// 			continue
	// 		}
	// 		statusSlice = append(statusSlice, entityStatus)
	// 	}
	// }
	err = w.app.GroupBroadcast(ctx, FrontEndType, WorldRoom, NewPlayer, &worldProto.PlayerJoin{
		Uuid:     s.UID(),
		Entities: statusSlice,
	})
	if err != nil {
		logger.Error("Failed to broadcast NewPlayer")
		logger.Error(err)
		return nil, err
	}
	return &worldProto.Response{Code: 200, Message: "ok"}, nil
}

func (w *World) Leave(ctx context.Context) (*worldProto.Response, error) {
	logger := w.logger
	s := w.app.GetSessionFromCtx(ctx)
	err := w.app.GroupRemoveMember(ctx, WorldRoom, s.UID())
	if err != nil {
		logger.Error("Failed to leave world")
		logger.Error(err)
		return nil, err
	}
	err = w.app.GroupBroadcast(ctx, FrontEndType, WorldRoom, PlayerQuit, &worldProto.PlayerQuit{Uuid: s.UID()})
	if err != nil {
		logger.Error("Failed to broadcast PlayerQuit")
		logger.Error(err)
		return nil, err
	}
	s.SetData(map[string]interface{}{
		"group": "",
	})
	return &worldProto.Response{Code: 200, Message: "ok"}, nil
}

// TODO: impl
func (w *World) AOIBoardCast(ctx context.Context, route string, msg *worldProto.EntityStatus) error {
	// uids, err := w.app.GroupMembers(ctx, WorldRoom)
	// if err != nil {
	// 	return err
	// }
	// for _uid := range uids {
	// 	location := getLocationOfUid
	// 	statusSlice := findNearestEntities(area, locations)
	// 	w.app.SendPushToUsers(route, msg, []string{uid}, FrontEndType)
	// }
	return nil
}

func (w *World) handleTick() {
	memberCount, _ := w.app.GroupCountMembers(context.Background(), WorldRoom)
	if memberCount <= 0 {
		return
	}
	objMgr := ObjectMgr{
		Ctx:       w.ctx,
		Id:        w.app.GetServerID(),
		ScopeName: w.getRegion(),
		Redis:     w.redis,
		StreamMgr: w.streamMgr,
		Logger:    w.logger,
	}
	objs := objMgr.Get()
	if len(objs) > 0 {
		ids := make([]string, 0)
		for _, obj := range objs {
			ids = append(ids, obj.GetId())
		}
		unSyncStatus, err := objMgr.ReadStream(ids)
		if err == nil && len(unSyncStatus) > 0 {
			snapshot := &worldProto.Snapshot{
				Rtt: &worldProto.RTT{
					Id:       time.Now().String(),
					StartMts: float64(time.Now().UnixMicro()) / 1000,
				},
				Entities: unSyncStatus,
			}
			// logger.Infof("Broadcast snapshots length=%d", len(snapshot.Entities))
			w.app.GroupBroadcast(w.ctx, FrontEndType, WorldRoom, "SyncSnapshot", snapshot)
		}
	}
}

func (w *World) syncLoop(ctx context.Context) {
	// maxConcurrency := runtime.NumCPU() * 2
	maxConcurrency := 1
	w.logger.Infof("Tick handler count=%d", maxConcurrency)
	tickCh := make(chan int, maxConcurrency)
	sampleLimit := 1000
	durations := make([]int64, sampleLimit)
	sampleIdx := 0
	var wg sync.WaitGroup
	var rwLock sync.RWMutex

	statTicker := time.NewTicker(1 * time.Second)
	go func(ctx context.Context) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-statTicker.C:
				rwLock.RLock()
				sum := int64(0)
				for _, duration := range durations {
					sum += duration
				}
				us := time.Duration(sum / int64(len(durations)))
				rwLock.RUnlock()
				w.logger.Infof("tick handler cost time avg=%v us", us.Milliseconds())
			}
		}
	}(w.ctx)
	wg.Add(1)

	for c := 0; c < maxConcurrency; c++ {
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tickCh:
					start := time.Now().UnixNano()
					w.handleTick()
					rwLock.Lock()
					durations[sampleIdx] = time.Now().UnixNano() - start
					sampleIdx++
					sampleIdx %= sampleLimit
					rwLock.Unlock()
				}
			}
		}(w.ctx)
	}
	wg.Add(maxConcurrency)
	ticker := time.NewTicker(16 * time.Millisecond)
	w.ticker = ticker

Loop:
	for {
		select {
		case <-ticker.C:
			tickCh <- 1
		case <-ctx.Done():
			break Loop
		}
	}
	wg.Wait()
	close(tickCh)
}

func (w *World) Rtt(ctx context.Context, rtt *worldProto.RTT) (*worldProto.RTTACK, error) {
	mtsNow := float64(time.Now().UnixMicro()) / 1000
	rttAck := &worldProto.RTTACK{
		Id:          rtt.Id,
		StartMts:    rtt.StartMts,
		ReceivedMts: mtsNow,
	}
	return rttAck, nil
}

func (w *World) RttAck(ctx context.Context, rttAck *worldProto.RTTACK) {
	session := w.app.GetSessionFromCtx(ctx)
	mtsNow := float64(time.Now().UnixMicro()) / 1000
	w.rttMap[session.UID()] = time.Duration(mtsNow - rttAck.StartMts)
}
