package models

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"scheduler0/pkg/protobuffs"

	"google.golang.org/protobuf/proto"
)

// AccountExhaustion is a notification from a worker node to the leader
// that an account's execution quota has been exhausted
type AccountExhaustion struct {
	AuthUsername string
	AuthPassword string
	AccountId    uint64
}

func (ae *AccountExhaustion) Bytes() []byte {
	accountExhaustionProtobuf := &protobuffs.AccountExhaustionPayload{
		AuthUsername: ae.AuthUsername,
		AuthPassword: ae.AuthPassword,
		AccountId:    ae.AccountId,
	}

	accountExhaustion, err := proto.Marshal(accountExhaustionProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal account exhaustion data")
	}

	return accountExhaustion
}

func (ae *AccountExhaustion) String() string {
	accountExhaustionProtobuf := &protobuffs.AccountExhaustionPayload{
		AuthUsername: ae.AuthUsername,
		AuthPassword: ae.AuthPassword,
		AccountId:    ae.AccountId,
	}

	accountExhaustion, err := proto.Marshal(accountExhaustionProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal account exhaustion data")
	}

	return string(accountExhaustion)
}

func (ae *AccountExhaustion) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, AccountExhaustionPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	accountExhaustionProtobuf := &protobuffs.AccountExhaustionPayload{
		AuthUsername: ae.AuthUsername,
		AuthPassword: ae.AuthPassword,
		AccountId:    ae.AccountId,
	}

	accountExhaustion, err := proto.Marshal(accountExhaustionProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal account exhaustion data")
		return n, err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(accountExhaustion)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(accountExhaustion)

	return n + int64(o), err
}

func (ae *AccountExhaustion) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != AccountExhaustionPayload {
		return n, errors.New("invalid AccountExhaustion payload")
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
	accountExhaustionProtobuf := &protobuffs.AccountExhaustionPayload{}
	err = proto.Unmarshal(buf, accountExhaustionProtobuf)
	if err != nil {
		return n, err
	}

	ae.AuthPassword = accountExhaustionProtobuf.AuthPassword
	ae.AuthUsername = accountExhaustionProtobuf.AuthUsername
	ae.AccountId = accountExhaustionProtobuf.AccountId

	return n + int64(o), nil
}
