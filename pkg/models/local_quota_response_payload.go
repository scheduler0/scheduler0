package models

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"scheduler0-private/pkg/protobuffs"

	"google.golang.org/protobuf/proto"
)

// LocalQuotaResponse is a response from a worker node containing its local quota allocations
type LocalQuotaResponse struct {
	AccountAllocations map[uint64]uint64 // accountId -> remainingCount
}

func (lqr *LocalQuotaResponse) Bytes() []byte {
	localQuotaResponseProtobuf := &protobuffs.LocalQuotaResponsePayload{
		AccountAllocations: lqr.AccountAllocations,
	}

	localQuotaResponse, err := proto.Marshal(localQuotaResponseProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota response data")
	}

	return localQuotaResponse
}

func (lqr *LocalQuotaResponse) String() string {
	localQuotaResponseProtobuf := &protobuffs.LocalQuotaResponsePayload{
		AccountAllocations: lqr.AccountAllocations,
	}

	localQuotaResponse, err := proto.Marshal(localQuotaResponseProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota response data")
	}

	return string(localQuotaResponse)
}

func (lqr *LocalQuotaResponse) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, LocalQuotaResponsePayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	localQuotaResponseProtobuf := &protobuffs.LocalQuotaResponsePayload{
		AccountAllocations: lqr.AccountAllocations,
	}

	localQuotaResponse, err := proto.Marshal(localQuotaResponseProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal local quota response data")
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(localQuotaResponse)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(localQuotaResponse)

	return n + int64(o), err
}

func (lqr *LocalQuotaResponse) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != LocalQuotaResponsePayload {
		return n, errors.New("invalid LocalQuotaResponse")
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
	localQuotaResponseProtobuf := &protobuffs.LocalQuotaResponsePayload{}
	err = proto.Unmarshal(buf, localQuotaResponseProtobuf)
	if err != nil {
		return n, err
	}

	lqr.AccountAllocations = localQuotaResponseProtobuf.AccountAllocations

	return n + int64(o), nil
}

