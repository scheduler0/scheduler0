package models

import (
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"scheduler0/pkg/protobuffs"
)

type NodeAuth struct {
	AuthUsername string
	AuthPassword string
}

func (nd *NodeAuth) Bytes() []byte {
	nodeAuthProtobuf := &protobuffs.NodeAuth{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal node auth data")
	}

	return nodeAuthDate
}

func (nd *NodeAuth) String() string {
	nodeAuthProtobuf := &protobuffs.NodeAuth{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal node auth data")
	}

	return string(nodeAuthDate)
}

func (nd *NodeAuth) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, NodeAuthPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	nodeAuthProtobuf := &protobuffs.NodeAuth{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal node auth data")
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(nodeAuthDate)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(nodeAuthDate)

	return n + int64(o), err
}

func (nd *NodeAuth) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != NodeAuthPayload {
		return n, errors.New("invalid node auth payload")
	}

	var size uint32
	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return n, err
	}
	n += 4
	if size > MaxPayloadSize {
		return n, ErrMaxPayloadSize
	}
	buf := make([]byte, size)
	o, err := r.Read(buf)
	if err != nil {
		return n, err
	}
	nodeAuthProtobuf := &protobuffs.NodeAuth{}
	err = proto.Unmarshal(buf, nodeAuthProtobuf)
	if err != nil {
		return n, err
	}

	nd.AuthPassword = nodeAuthProtobuf.AuthPassword
	nd.AuthUsername = nodeAuthProtobuf.AuthUsername

	return n + int64(o), nil
}
