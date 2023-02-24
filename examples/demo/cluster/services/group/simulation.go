package group

import "errors"

type Simulator interface {
	Simulate(prevData interface{}, in Event) (out Event, err error)
}

type Router interface {
	AddRoute(channel, event string, pipeline Simulator) error
	Route(event Event) (Simulator, error)
}

type SimulationLoop interface {
	// GetEventsOnTick(tick uint64) []Event
	Run()
	Close() error
	GetRouter() Router
	GetEventLoop() EventLoop
}

type Storage interface {
	Put(key, value interface{})
	Get(key interface{})
}

type SimpleRouter struct {
	route map[string]Simulator
}

func NewSimpleRouter() Router {
	return &SimpleRouter{
		route: make(map[string]Simulator),
	}
}

func (r *SimpleRouter) AddRoute(channel, event string, pipeline Simulator) error {
	key := channel + "." + event
	r.route[key] = pipeline
	return nil
}

func (r *SimpleRouter) Route(event Event) (Simulator, error) {
	key := event.GetEventName()
	if pipeline, ex := r.route[key]; ex {
		return pipeline, nil
	}
	return nil, errors.New("Route not found")
}

// TODO: set storage
type MemorySimulationLoop struct {
	startTick uint64
	tickRate  uint64
	loop      EventLoop
	router    Router
}

func (sl *MemorySimulationLoop) Run() {
	sl.loop.Run(sl.startTick, sl.tickRate)
}

func (sl *MemorySimulationLoop) Close() error {
	return sl.loop.Close()
}

func (sl *MemorySimulationLoop) GetRouter() Router {
	return sl.router
}

func (sl *MemorySimulationLoop) GetEventLoop() EventLoop {
	return sl.loop
}

func NewMemorySimulationLoop(startTick, tickRate uint64) SimulationLoop {
	return &MemorySimulationLoop{
		startTick: startTick,
		tickRate:  tickRate,
		loop:      NewMemoryEventLoop(),
		router:    NewSimpleRouter(),
	}
}
