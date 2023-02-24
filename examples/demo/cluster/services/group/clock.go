package group

import "time"

type Clock interface {
	Run(tickRate uint64, onTick OnTickFunc)
	Close() error
	GetTick() uint64
}

type MemoryClock struct {
	tick   uint64
	quitCh chan int
}

func NewMemoryClock() *MemoryClock {
	quitCh := make(chan int)
	clock := &MemoryClock{
		tick:   0,
		quitCh: quitCh,
	}
	return clock
}

type OnTickFunc func(tick uint64)

func (c *MemoryClock) Run(tickRate uint64, onTick OnTickFunc) {
	c.tick = 0
	driver := time.NewTimer(time.Second / time.Duration(tickRate))
	for {
		select {
		case <-driver.C:
			c.tick++
			onTick(c.tick)
		case <-c.quitCh:
			driver.Stop()
			return
		}
	}
}

func (c *MemoryClock) GetTick() uint64 {
	return c.tick
}

func (c *MemoryClock) Close() error {
	c.quitCh <- 1
	return nil
}
