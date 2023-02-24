// Copyright (c) nano Author and TFG Co. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package protobuf

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/topfreegames/pitaya/v2/constants"
	worldPb "github.com/topfreegames/pitaya/v2/examples/demo/cluster/proto"
	"github.com/topfreegames/pitaya/v2/helpers"
	"github.com/topfreegames/pitaya/v2/protos"

	// "google.golang.org/protobuf/proto"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

var update = flag.Bool("update", false, "update .golden files")

func TestNewSerializer(t *testing.T) {
	t.Parallel()
	serializer := NewSerializer()
	assert.NotNil(t, serializer)
}

func TestMarshal(t *testing.T) {
	var marshalTables = map[string]struct {
		raw interface{}
		err error
	}{
		"test_ok":            {&protos.Response{Data: []byte("data"), Error: &protos.Error{Msg: "error"}}, nil},
		"test_not_a_message": {"invalid", constants.ErrWrongValueType},
		"test_pboneof": {
			&worldPb.SyncAction{
				Uuid: "1",
				Type: worldPb.ActionType_MOVE,
			},
			nil,
		},
	}
	serializer := NewSerializer()

	for name, table := range marshalTables {
		t.Run(name, func(t *testing.T) {
			result, err := serializer.Marshal(table.raw)
			gp := helpers.FixtureGoldenFileName(t, t.Name())

			if table.err == nil {
				assert.NoError(t, err)
				if *update {
					t.Log("updating golden file")
					helpers.WriteFile(t, gp, result)
				}
				expected := helpers.ReadFile(t, gp)
				assert.Equal(t, expected, result)
			} else {
				assert.Equal(t, table.err, err)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	gp := helpers.FixtureGoldenFileName(t, "TestMarshal/test_ok")
	data := helpers.ReadFile(t, gp)

	var dest protos.Response
	var unmarshalTables = map[string]struct {
		expected interface{}
		data     []byte
		dest     interface{}
		err      error
	}{
		"test_ok":           {&protos.Response{Data: []byte("data"), Error: &protos.Error{Msg: "error"}}, data, &dest, nil},
		"test_invalid_dest": {&protos.Response{Data: []byte(nil)}, data, "invalid", constants.ErrWrongValueType},
	}
	serializer := NewSerializer()

	for name, table := range unmarshalTables {
		t.Run(name, func(t *testing.T) {
			result := table.dest
			err := serializer.Unmarshal(table.data, result)
			assert.Equal(t, table.err, err)
			if table.err == nil {
				assert.Equal(t, table.expected, result)
			}
		})
	}
}

func TestGoogleProtoBuffer(t *testing.T) {
	uuid := "2"

	var unmarshalTables = map[string]struct {
		expected interface{}
	}{
		"test_pboneof": {
			&worldPb.SyncAction{
				Uuid: "1",
				Type: worldPb.ActionType_MOVE,
				Payload: &worldPb.SyncAction_Move{
					Move: &worldPb.MoveObject{
						Uuid: &uuid,
					},
				},
			},
		},
		// "test_simple": {
		// 	&worldPb.Response{
		// 		Code:    100,
		// 		Message: "ok",
		// 	},
		// },
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
