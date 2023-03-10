package simulate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

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
	reqBuf    []*worldProto.EntityStatus
	reqLock   sync.RWMutex
}

func NewSimulateComponent(streamMgr storage.StreamManager, app pitaya.Pitaya, redis *redis.Client) *SimulateComponent {
	return &SimulateComponent{streamMgr: streamMgr, app: app, s: jsonpb.NewSerializer(), redis: redis}
}

func (s *SimulateComponent) Init() {

}

func (s *SimulateComponent) AfterInit() {

}

func (s *SimulateComponent) BeforeShutdown() {

}

func (s *SimulateComponent) Shutdown() {

}

func addVector3(a, b *worldProto.Vector3) *worldProto.Vector3 {
	result := &worldProto.Vector3{}
	x := a.GetX() + b.GetX()
	y := a.GetY() + b.GetY()
	z := a.GetZ() + b.GetZ()
	result.X = &x
	result.Y = &y
	result.Z = &z
	return result
}

func (s *SimulateComponent) Call(ctx context.Context, msg *worldProto.SimulateRPCRequest) (*worldProto.SimulateRPCTicket, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	switch msg.Method {
	case worldProto.SimulateRPCMethod_name[int32(worldProto.SimulateRPCMethod_MOVE)]:
		moveStatus := &worldProto.EntityStatus{}
		err := s.s.Unmarshal(msg.GetRawArgs()[0], moveStatus)
		if err != nil {
			logger.Error(err)
			return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
		}
		resultStreamId := fmt.Sprintf("%s:%s:%s:%s", s.app.GetServer().Type, s.app.GetServer().ID, "simulator:result", moveStatus.Id)
		resultStream := s.streamMgr.CreateStream(resultStreamId)
		msgs, err := resultStream.Read(ctx, strconv.FormatUint(msg.ClientTick, 10), "status")
		if err != nil {
			logger.Error(err)
			return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
		}
		beforeMove := &worldProto.EntityStatus{}
		if len(msgs) == 0 {

		} else {
			statusBytes := msgs[len(msgs)-1].GetValue("status", nil)
			if statusBytes == nil {
				logger.Error(errors.New("can not read status bytes"))
			}
			err := s.s.Unmarshal(statusBytes.([]byte), beforeMove)
			if err != nil {
				logger.Error(err)
				return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
			}
		}
		afterMove := &worldProto.EntityStatus{}
		afterMove.Position = addVector3(beforeMove.GetPosition(), moveStatus.GetPosition())
		afterMove.Rotation = addVector3(beforeMove.GetRotation(), moveStatus.GetRotation())
		afterMove.Scale = addVector3(beforeMove.GetScale(), moveStatus.GetScale())
		s.app.GetServerID()
		statusBytes, err := s.s.Marshal(afterMove)
		if err != nil {
			logger.Error(err)
			return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
		}
		resultValues := make(map[string]interface{})
		resultValues["status"] = statusBytes
		resultValues["tick"] = msg.ClientTick + 1
		redisXMsg := redis.XMessage{
			ID:     strconv.FormatUint(msg.ClientTick+1, 10),
			Values: resultValues,
		}
		redisStreamMessage := &storage.RedisStreamMessage{XMessage: redisXMsg}
		resultMsgId, err := resultStream.Add(ctx, redisStreamMessage)
		if err != nil {
			logger.Error(err)
			return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
		}
		responseStreamId := fmt.Sprintf("%s:%s:%s", s.app.GetServer().Type, "simulator:response", moveStatus.Id)
		responseStream := s.streamMgr.CreateStream(responseStreamId)
		values := make(map[string]interface{})
		values["status"] = statusBytes
		values["tick"] = msg.ClientTick + 1
		values["requestId"] = resultMsgId
		redisXMsg = redis.XMessage{
			ID:     "*",
			Values: values,
		}
		redisStreamMessage = &storage.RedisStreamMessage{XMessage: redisXMsg}
		responseStream.Add(ctx, redisStreamMessage)
		s.redis.SAdd(ctx, "world:sync:entity", moveStatus.Id)
		return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_OK), RequestId: &resultMsgId}, nil
	default:
		return &worldProto.SimulateRPCTicket{Code: uint32(worldProto.SimulateRPCTicketCode_FAILED)}, nil
	}
}

func (s *SimulateComponent) Forward(ctx context.Context, msg *worldProto.EntityStatus) (*worldProto.EntityStatus, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	statusBytes, err := s.s.Marshal(msg)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	// resultStreamId := fmt.Sprintf("%s:%s:%s", s.app.GetServer().Type, "simulator:result", msg.Id)
	resultStreamId := fmt.Sprintf("%s:%s:%s:%s", s.app.GetServer().Type, s.app.GetServer().ID, "simulator:result", msg.Id)
	s.redis.SAdd(ctx, world.EntityStoreKey, msg.Id)
	resultStream := s.streamMgr.CreateStream(resultStreamId)
	resultValues := make(map[string]interface{})
	resultValues["status"] = statusBytes
	redisXMsg := redis.XMessage{
		ID:     "*",
		Values: resultValues,
	}
	redisStreamMessage := &storage.RedisStreamMessage{XMessage: redisXMsg}
	_, err = resultStream.Add(ctx, redisStreamMessage)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	session := s.app.GetSessionFromCtx(ctx)
	sessionData := session.GetData()
	if groupName, ex := sessionData["group"]; ex && groupName.(string) != "" {
		key := fmt.Sprintf("room:%s:laststatus:%s", s.app.GetServerID(), msg.GetId())
		s.redis.Set(ctx, key, statusBytes, 30*time.Second)
	}

	return msg, nil
}

// func (s *SimulateComponent) Test(ctx context.Context) (*worldProto.EntityStatus, error) {
// 	absFilePath, err := filepath.Abs("./resouces/Shall016.glb")
// 	if err != nil {
// 		return nil, err
// 	}
// 	gltfDoc, err := gltf.ParseBin(absFilePath)
// 	if err != nil {
// 		return nil, err
// 	}

// 	gltfDoc.Accessors
// 	gltfDoc.Animations
// 	gltfDoc.Asset
// 	gltfDoc.Cameras
// 	gltfDoc.Scene
// 	gltfDoc.
// 	fmt.Println(gltfDoc)
// 	return &worldProto.EntityStatus{Id: "1"}, nil
// }
