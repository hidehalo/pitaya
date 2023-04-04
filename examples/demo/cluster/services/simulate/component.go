package simulate

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	storage "github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/storage"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/world"
	"github.com/topfreegames/pitaya/v2/serialize"
	"github.com/topfreegames/pitaya/v2/serialize/jsonpb"
)

type SimulateComponent struct {
	component.Component
	streamMgr storage.StreamManager
	app       pitaya.Pitaya
	s         serialize.Serializer
	redis     *redis.Client
	region    string
}

func NewSimulateComponent(streamMgr storage.StreamManager, app pitaya.Pitaya, redis *redis.Client) *SimulateComponent {
	return &SimulateComponent{
		streamMgr: streamMgr,
		app:       app,
		s:         jsonpb.NewSerializer(),
		redis:     redis,
		region:    fmt.Sprintf("%s:%s:%s", world.WorldRoom, app.GetServer().Type, app.GetServer().ID),
	}
}

func (s *SimulateComponent) Init() {

}

func (s *SimulateComponent) AfterInit() {

}

func (s *SimulateComponent) BeforeShutdown() {

}

func (s *SimulateComponent) Shutdown() {

}

func (s *SimulateComponent) getRegion() string {
	return s.region
}

func (s *SimulateComponent) Forward(ctx context.Context, msg *worldProto.EntityStatus) (*worldProto.EntityStatus, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	objMgr := world.ObjectMgr{
		Ctx:       ctx,
		Id:        s.app.GetServerID(),
		ScopeName: s.getRegion(),
		Redis:     s.redis,
		StreamMgr: s.streamMgr,
		Logger:    logger,
	}
	err := objMgr.PutStream(msg)
	if err != nil {
		return nil, err
	}
	err = objMgr.Add(msg)
	if err != nil {
		return nil, err
	}
	session := s.app.GetSessionFromCtx(ctx)
	sessionData := session.GetData()
	if groupName, ex := sessionData["group"]; ex && groupName.(string) != "" {
		err := objMgr.PutLastStatus(msg)
		if err != nil {
			return nil, err
		}
	}
	return msg, nil

	// statusBytes, err := s.s.Marshal(msg)
	// if err != nil {
	// 	logger.Error(err)
	// 	return nil, err
	// }
	// // resultStreamId := fmt.Sprintf("%s:%s:%s", s.app.GetServer().Type, "simulator:result", msg.Id)
	// resultStreamId := fmt.Sprintf("%s:%s:%s:%s", s.app.GetServer().Type, s.app.GetServer().ID, "simulator:result", msg.Id)
	// s.redis.SAdd(ctx, world.EntityStoreKey, msg.Id)
	// resultStream := s.streamMgr.CreateStream(resultStreamId)
	// resultValues := make(map[string]interface{})
	// resultValues["status"] = statusBytes
	// redisXMsg := redis.XMessage{
	// 	ID:     "*",
	// 	Values: resultValues,
	// }
	// redisStreamMessage := &storage.RedisStreamMessage{XMessage: redisXMsg}
	// _, err = resultStream.Add(ctx, redisStreamMessage)
	// if err != nil {
	// 	logger.Error(err)
	// 	return nil, err
	// }
	// session := s.app.GetSessionFromCtx(ctx)
	// sessionData := session.GetData()
	// if groupName, ex := sessionData["group"]; ex && groupName.(string) != "" {
	// 	key := fmt.Sprintf("room:%s:laststatus:%s", s.app.GetServerID(), msg.GetId())
	// 	s.redis.Set(ctx, key, statusBytes, 30*time.Second)
	// }

	// return msg, nil
}
