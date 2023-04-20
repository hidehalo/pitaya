package simulate

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/aoi"
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
	lbs       *aoi.LBS
}

func NewSimulateComponent(streamMgr storage.StreamManager, app pitaya.Pitaya, redis *redis.Client, lbs *aoi.LBS) *SimulateComponent {
	return &SimulateComponent{
		streamMgr: streamMgr,
		app:       app,
		s:         jsonpb.NewSerializer(),
		redis:     redis,
		region:    fmt.Sprintf("%s:%s:%s", world.WorldRoom, app.GetServer().Type, app.GetServer().ID),
		lbs:       lbs,
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
	if lastLoc, ex := sessionData["lastLoc"]; ex && lastLoc != nil {
		lastLocF := lastLoc.([2]float64)
		s.lbs.ReplaceLocation(
			aoi.Location{X: lastLocF[0], Y: lastLocF[1]},
			aoi.Location{X: msg.GetPosition().GetX(), Y: msg.GetPosition().GetY()},
			session.UID(),
			session.UID())
	} else {
		s.lbs.AddLocation(aoi.Location{X: msg.GetPosition().GetX(), Y: msg.GetPosition().GetY()}, session.UID())
	}
	session.SetData(map[string]interface{}{
		"lastLoc": [2]float64{msg.GetPosition().GetX(), msg.GetPosition().GetY()},
	})
	return msg, nil
}
