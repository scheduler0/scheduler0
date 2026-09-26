package models

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"scheduler0-private/pkg/protobuffs"

	"google.golang.org/protobuf/proto"
)

// LocalQuotaRequest is a request from the leader to get local quota allocations from a worker node
type LocalQuotaRequest struct {
	AuthUsername string
	AuthPassword string
}

func (lqr *LocalQuotaRequest) Bytes() []byte {
	localQuotaRequestProtobuf := &protobuffs.LocalQuotaRequestPayload{
		AuthUsername: lqr.AuthUsername,
		AuthPassword: lqr.AuthPassword,
	}

	localQuotaRequest, err := proto.Marshal(localQuotaRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota request data")
	}

	return localQuotaRequest
}

func (lqr *LocalQuotaRequest) String() string {
	localQuotaRequestProtobuf := &protobuffs.LocalQuotaRequestPayload{
		AuthUsername: lqr.AuthUsername,
		AuthPassword: lqr.AuthPassword,
	}

	localQuotaRequest, err := proto.Marshal(localQuotaRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota request data")
	}

	return string(localQuotaRequest)
}

func (lqr *LocalQuotaRequest) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, LocalQuotaRequestPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	localQuotaRequestProtobuf := &protobuffs.LocalQuotaRequestPayload{
		AuthUsername: lqr.AuthUsername,
		AuthPassword: lqr.AuthPassword,
	}

	localQuotaRequest, err := proto.Marshal(localQuotaRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota request data")
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(localQuotaRequest)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(localQuotaRequest)

	return n + int64(o), err
}

func (lqr *LocalQuotaRequest) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != LocalQuotaRequestPayload {
		return n, errors.New("invalid LocalQuotaRequest")
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
	localQuotaRequestProtobuf := &protobuffs.LocalQuotaRequestPayload{}
	err = proto.Unmarshal(buf, localQuotaRequestProtobuf)
	if err != nil {
		return n, err
	}

	lqr.AuthPassword = localQuotaRequestProtobuf.AuthPassword
	lqr.AuthUsername = localQuotaRequestProtobuf.AuthUsername

	return n + int64(o), nil
}

