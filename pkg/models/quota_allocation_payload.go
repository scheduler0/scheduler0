package models

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"scheduler0/pkg/protobuffs"

	"google.golang.org/protobuf/proto"
)

type QuotaAllocation struct {
	AuthUsername       string
	AuthPassword       string
	AccountAllocations map[uint64]uint64 // accountId -> allocatedCount
}

func (qa *QuotaAllocation) Bytes() []byte {
	// Convert map to protobuf map
	accountAllocations := make(map[uint64]uint64)
	for k, v := range qa.AccountAllocations {
		accountAllocations[k] = v
	}

	quotaAllocationProtobuf := &protobuffs.QuotaAllocationPayload{
		AuthUsername:       qa.AuthUsername,
		AuthPassword:       qa.AuthPassword,
		AccountAllocations: accountAllocations,
	}

	quotaAllocation, err := proto.Marshal(quotaAllocationProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal quota allocation data")
	}

	return quotaAllocation
}

func (qa *QuotaAllocation) String() string {
	accountAllocations := make(map[uint64]uint64)
	for k, v := range qa.AccountAllocations {
		accountAllocations[k] = v
	}

	quotaAllocationProtobuf := &protobuffs.QuotaAllocationPayload{
		AuthUsername:       qa.AuthUsername,
		AuthPassword:       qa.AuthPassword,
		AccountAllocations: accountAllocations,
	}

	quotaAllocation, err := proto.Marshal(quotaAllocationProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal quota allocation data")
	}

	return string(quotaAllocation)
}

func (qa *QuotaAllocation) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, QuotaAllocationPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	accountAllocations := make(map[uint64]uint64)
	for k, v := range qa.AccountAllocations {
		accountAllocations[k] = v
	}

	quotaAllocationProtobuf := &protobuffs.QuotaAllocationPayload{
		AuthUsername:       qa.AuthUsername,
		AuthPassword:       qa.AuthPassword,
		AccountAllocations: accountAllocations,
	}

	quotaAllocation, err := proto.Marshal(quotaAllocationProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal quota allocation data")
		return n, err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(quotaAllocation)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(quotaAllocation)

	return n + int64(o), err
}

func (qa *QuotaAllocation) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != QuotaAllocationPayload {
		return n, errors.New("invalid quota allocation payload")
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
	quotaAllocationProtobuf := &protobuffs.QuotaAllocationPayload{}
	err = proto.Unmarshal(buf, quotaAllocationProtobuf)
	if err != nil {
		return n, err
	}

	qa.AuthPassword = quotaAllocationProtobuf.AuthPassword
	qa.AuthUsername = quotaAllocationProtobuf.AuthUsername
	qa.AccountAllocations = quotaAllocationProtobuf.AccountAllocations

	return n + int64(o), nil
}
