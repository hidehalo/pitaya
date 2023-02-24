package group

import (
	"sync"
	"time"
)

type EventLoop interface {
	Run(startTick, tickRate uint64)
	Close() error
	Register(event string, handler EventHandler)
	BroadCast(event Event)
	GetTickRate() uint64
	GetTick() uint64
}

type EventHandler func(event Event) error

type EventListener interface {
	Loop() EventLoop
	Listen(event Event, handler EventHandler)
}

type Event struct {
	Name      string
	Tick      uint64
	ClientMts float64
	ServerMts float64
	Data      interface{}
}

func (e *Event) GetEventName() string {
	return e.Name
}

func (e *Event) GetTick() uint64 {
	return e.Tick
}

func (e *Event) GetClientMts() float64 {
	return e.ClientMts
}

func (e *Event) GetServerMts() float64 {
	return e.ServerMts
}

func (e *Event) GetData() interface{} {
	return e.Data
}

type MemoryEventLoop struct {
	sync.Mutex
	startTick uint64
	tickRate  uint64
	route     map[string][]EventHandler
	clock     Clock
}

func (l *MemoryEventLoop) Run(startTick, tickRate uint64) {
	l.Lock()
	l.startTick = startTick
	l.tickRate = tickRate
	l.clock = NewMemoryClock()
	l.clock.Run(tickRate, func(tick uint64) {
		l.BroadCast(Event{
			Name:      "tick",
			Tick:      tick,
			ServerMts: float64(time.Now().UnixNano()) / 1000,
		})
	})
}

func (l *MemoryEventLoop) Close() error {
	defer l.Unlock()
	return l.clock.Close()
}

func (l *MemoryEventLoop) Register(event string, handler EventHandler) {
	key := event
	if _, ex := l.route[key]; !ex {
		l.route[key] = make([]EventHandler, 0)
	}
	l.route[key] = append(l.route[key], handler)
}

func (l *MemoryEventLoop) BroadCast(event Event) {
	key := event.GetEventName()
	if _, ex := l.route[key]; ex {
		for _, handler := range l.route[key] {
			handler(event)
		}
	}
}

func (l *MemoryEventLoop) GetTickRate() uint64 {
	return l.tickRate
}

func (l *MemoryEventLoop) GetTick() uint64 {
	return l.startTick + l.clock.GetTick()
}

func NewMemoryEventLoop() EventLoop {
	return &MemoryEventLoop{
		route: make(map[string][]EventHandler),
	}
}
