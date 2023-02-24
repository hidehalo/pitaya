package world

import (
	"context"
	"strconv"
	"time"

	pbProto "github.com/golang/protobuf/proto"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	worldProto "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/group"
	"github.com/topfreegames/pitaya/v2/examples/demo/protos"
)

const (
	NewPlayer    string = "NewPlayer"
	PlayerQuit   string = "PlayerQuit"
	SyncAction   string = "SyncAction"
	GroupMembers string = "GroupMembers"
)

const WorldRoom string = "world"

type World struct {
	component.Base
	app  pitaya.Pitaya
	loop group.SimulationLoop
}

func NewWorld(app pitaya.Pitaya) *World {
	return &World{
		app:  app,
		loop: group.NewMemorySimulationLoop(0, 60),
	}
}

func (w *World) Init() {
	w.app.GroupCreate(context.Background(), WorldRoom)
	w.loop.GetRouter().AddRoute(WorldRoom, "move", &MoveSimulator{})
	w.loop.GetEventLoop().Register("tick", func(event group.Event) error {
		go w.app.GroupBroadcast(context.Background(), "connector", WorldRoom, "syncStatus", event)
		return nil
	})

	go w.loop.Run()
}

func (w *World) AfterInit() {

}

func (w *World) BeforeShutdown() {
	w.loop.Close()
}

func (w *World) Shutdown() {
	w.app.GroupDelete(context.Background(), WorldRoom)
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

func (w *World) BroadCastRaw(ctx context.Context, msg *worldProto.RawMessage) {
	w.sendPushToOthers(ctx, "BroadCastRaw", msg)
}

// func (w *World) Docs(ctx context.Context) (*protos.JoinResponse, error) {
// 	docs, _ := w.app.Documentation(true)
// 	return &protos.JoinResponse{Result: fmt.Sprintf("%v", docs)}, nil
// }
