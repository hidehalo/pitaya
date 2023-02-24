package jsonpb

import (
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/topfreegames/pitaya/v2/constants"
)

// Serializer implements the serialize.Serializer interface
type Serializer struct{}

// NewSerializer returns a new Serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// Marshal returns the JSON encoding of v.
func (s *Serializer) Marshal(v interface{}) ([]byte, error) {
	pb, ok := v.(proto.Message)
	if !ok {
		return nil, constants.ErrWrongValueType
	}
	marshaler := &jsonpb.Marshaler{}
	jsonStr, err := marshaler.MarshalToString(pb)
	return []byte(jsonStr), err
}

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v.
func (s *Serializer) Unmarshal(data []byte, v interface{}) error {
	pb, ok := v.(proto.Message)
	if !ok {
		return constants.ErrWrongValueType
	}
	return jsonpb.UnmarshalString(string(data), pb)
}

// GetName returns the name of the serializer.
func (s *Serializer) GetName() string {
	return "jsonpb"
}
