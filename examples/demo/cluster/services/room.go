package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/component"
	"github.com/topfreegames/pitaya/v2/examples/demo/protos"
	"github.com/topfreegames/pitaya/v2/timer"
)

type (
	// Room represents a component that contains a bundle of room related handler
	// like Join/Message
	Room struct {
		component.Base
		timer *timer.Timer
		app   pitaya.Pitaya
		Stats *Stats
	}

	// UserMessage represents a message that user sent
	UserMessage struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	// Stats exports the room status
	Stats struct {
		outboundBytes int
		inboundBytes  int
	}

	// RPCResponse represents a rpc message
	RPCResponse struct {
		Msg string `json:"msg"`
	}

	// SendRPCMsg represents a rpc message
	SendRPCMsg struct {
		ServerID string `json:"serverId"`
		Route    string `json:"route"`
		Msg      string `json:"msg"`
	}

	// NewUser message will be received when new user join room
	NewUser struct {
		Content string `json:"content"`
	}

	// AllMembers contains all members uid
	AllMembers struct {
		Members []string `json:"members"`
	}

	// JoinResponse represents the result of joining room
	JoinResponse struct {
		Code   int    `json:"code"`
		Result string `json:"result"`
	}
)

// Outbound gets the outbound status
func (Stats *Stats) Outbound(ctx context.Context, in []byte) ([]byte, error) {
	Stats.outboundBytes += len(in)
	return in, nil
}

// Inbound gets the inbound status
func (Stats *Stats) Inbound(ctx context.Context, in []byte) ([]byte, error) {
	Stats.inboundBytes += len(in)
	return in, nil
}

// NewRoom returns a new room
func NewRoom(app pitaya.Pitaya) *Room {
	return &Room{
		app:   app,
		Stats: &Stats{},
	}
}

// Init runs on service initialization
func (r *Room) Init() {
	r.app.GroupCreate(context.Background(), "room")
}

// AfterInit component lifetime callback
func (r *Room) AfterInit() {
	r.timer = pitaya.NewTimer(time.Minute, func() {
		count, err := r.app.GroupCountMembers(context.Background(), "room")
		println("UserCount: Time=>", time.Now().String(), "Count=>", count, "Error=>", err)
		println("OutboundBytes", r.Stats.inboundBytes)
		println("InboundBytes", r.Stats.outboundBytes)
	})
}

// Entry is the entrypoint
func (r *Room) Entry(ctx context.Context, msg []byte) (*protos.JoinResponse, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx) // The default logger contains a requestId, the route being executed and the sessionId
	s := r.app.GetSessionFromCtx(ctx)

	err := s.Bind(ctx, "banana")
	if err != nil {
		logger.Error("Failed to bind session")
		logger.Error(err)
		return nil, pitaya.Error(err, "RH-000", map[string]string{"failed": "bind"})
	}
	return &protos.JoinResponse{Result: "ok"}, nil
}

// GetSessionData gets the session data
func (r *Room) GetSessionData(ctx context.Context) (*SessionData, error) {
	s := r.app.GetSessionFromCtx(ctx)
	return &SessionData{
		Data: s.GetData(),
	}, nil
}

// SetSessionData sets the session data
func (r *Room) SetSessionData(ctx context.Context, data *SessionData) ([]byte, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	s := r.app.GetSessionFromCtx(ctx)
	err := s.SetData(data.Data)
	if err != nil {
		logger.Error("Failed to set session data")
		logger.Error(err)
		return nil, err
	}
	err = s.PushToFront(ctx)
	if err != nil {
		return nil, err
	}
	return []byte("success"), nil
}

// Join room
func (r *Room) Join(ctx context.Context) (*protos.JoinResponse, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	s := r.app.GetSessionFromCtx(ctx)
	fakeUID := s.ID()
	s.Bind(ctx, strconv.Itoa(int(fakeUID))) // binding session uid
	err := r.app.GroupAddMember(ctx, "room", s.UID())
	if err != nil {
		logger.Error("Failed to join room")
		logger.Error(err)
		return nil, err
	}
	members, err := r.app.GroupMembers(ctx, "room")
	if err != nil {
		logger.Error("Failed to get members")
		logger.Error(err)
		return nil, err
	}
	allMemMsg := protos.AllMembers{Members: members}
	err = s.Push("onMembers", &allMemMsg)
	if err != nil {
		r.Stats.Outbound(ctx, []byte(allMemMsg.String()))
	}
	err = r.app.GroupBroadcast(ctx, "connector", "room", "onNewUser", &protos.NewUser{Content: fmt.Sprintf("New user: %d", s.ID())})
	if err != nil {
		logger.Error("Failed to broadcast onNewUser")
		logger.Error(err)
		return nil, err
	}
	err = s.OnClose(func() {
		r.app.GroupRemoveMember(ctx, "room", s.UID())
	})
	if err != nil {
		return nil, err
	}
	return &protos.JoinResponse{Result: "success"}, nil
}

// Message sync last message to all members
func (r *Room) Message(ctx context.Context, msg *protos.UserMessage) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	err := r.app.GroupBroadcast(ctx, "connector", "room", "onMessage", msg)
	if err != nil {
		logger.Error("Error broadcasting message")
		logger.Error(err)
	}
	count, err := r.app.GroupCountMembers(context.Background(), "room")
	if err != nil {
		logger.Error("Error counting bound")
		logger.Error(err)
	}
	boundBytes := []byte(msg.String())
	r.Stats.Inbound(ctx, boundBytes)
	for i := 0; i < count; i++ {
		r.Stats.Outbound(ctx, boundBytes)
	}
}

func (r *Room) Docs(ctx context.Context) (*protos.JoinResponse, error) {
	docs, _ := r.app.Documentation(true)
	return &protos.JoinResponse{Result: fmt.Sprintf("%v", docs)}, nil
}

// SendRPC sends rpc
func (r *Room) SendRPC(ctx context.Context, msg *protos.SendRPCMsg) (*protos.RPCRes, error) {
	logger := pitaya.GetDefaultLoggerFromCtx(ctx)
	ret := &protos.RPCRes{}
	err := r.app.RPCTo(ctx, msg.ServerId, msg.Route, ret, &protos.RPCMsg{Msg: msg.Msg})
	if err != nil {
		logger.Errorf("Failed to execute RPCTo %s - %s", msg.ServerId, msg.Route)
		logger.Error(err)
		return nil, pitaya.Error(err, "RPC-000")
	}
	return ret, nil
}

// MessageRemote just echoes the given message
func (r *Room) MessageRemote(ctx context.Context, msg *protos.UserMessage) (*protos.UserMessage, error) {
	return msg, nil
}
