package jsonpb

import (
	"flag"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	worldPb "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
)

var update = flag.Bool("update", false, "update .golden files")

func TestNewSerializer(t *testing.T) {
	t.Parallel()
	serializer := NewSerializer()
	assert.NotNil(t, serializer)
}

func TestGoogleProtoBuffer(t *testing.T) {
	uuid := "2"

	var unmarshalTables = map[string]struct {
		expected interface{}
	}{
		"test_pboneof": {
			&worldPb.SyncAction{
				Uuid: "1",
				Type: worldPb.ActionType_PLAY_ANIMATION,
				Payload: &worldPb.SyncAction_PlayAnim{
					PlayAnim: &worldPb.PlayAnimation{
						Uuid:          &uuid,
						AnimationUuid: &uuid,
					},
				},
			},
		},
	}
	var unMarshalled worldPb.SyncAction
	for name, table := range unmarshalTables {
		t.Run(name, func(t *testing.T) {
			pb, ok := table.expected.(proto.Message)
			if !ok {
				panic("pb convert error")
			}
			marshaller := jsonpb.Marshaler{}
			marshalled, err := marshaller.MarshalToString(pb)
			assert.Equal(t, nil, err)
			err = jsonpb.UnmarshalString(marshalled, &unMarshalled)
			assert.Equal(t, nil, err)
			assert.Equal(t, table.expected, &unMarshalled)
		})
	}
}

func TestNested(t *testing.T) {
	marshaller := jsonpb.Marshaler{}
	pb := &worldPb.SimulateRPCRequest{
		Method:     worldPb.SimulateRPCMethod_name[int32(worldPb.SimulateRPCMethod_MOVE)],
		ClientTick: 1,
		ClientMts:  float64(time.Now().UnixMicro()),
		RawArgs: [][]byte{
			[]byte(`{"position":{"x":84,"y":426,"z":0}}`),
		},
	}
	marshalled, err := marshaller.MarshalToString(pb)
	assert.Equal(t, nil, err)
	upb := &worldPb.SimulateRPCRequest{}
	err = jsonpb.UnmarshalString(marshalled, upb)
	assert.Equal(t, nil, err)
	assert.Equal(t, pb, upb)
}
