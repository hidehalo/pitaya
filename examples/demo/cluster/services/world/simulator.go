package world

import "github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/group"

type MoveSimulator struct {
	group.Simulator
}

func (s *MoveSimulator) Simulate(prevData interface{}, in group.Event) (out group.Event, err error) {
	if prevData == nil {
		return in, nil
	}
	return in, nil
}
