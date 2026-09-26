package models

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"scheduler0-private/pkg/protobuffs"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AsyncTaskState uint64

const (
	AsyncTaskNotStated  AsyncTaskState = 0
	AsyncTaskInProgress AsyncTaskState = 1
	AsyncTaskSuccess    AsyncTaskState = 2
	AsyncTaskFail       AsyncTaskState = 3
)

type AsyncTask struct {
	Id           uint64         `json:"id" fake:"{number:1,100}"`
	RequestId    string         `json:"requestId" fake:"{word}"`
	Input        string         `json:"input" fake:"{word}"`
	Output       string         `json:"output" fake:"{word}"`
	Service      string         `json:"service" fake:"{word}"`
	State        AsyncTaskState `json:"state" fake:"{number:1,3}"`
	DateCreated  time.Time      `json:"dateCreated" fake:"{date}"`
	AccountId    uint64         `json:"accountId" fake:"{number:1,100}"`
	DateModified time.Time      `json:"dateModified" fake:"{date}"`
}

type AsyncTaskRes struct {
	Data    AsyncTask `json:"data"`
	Success bool      `json:"success"`
}

func (nd *AsyncTask) Bytes() []byte {
	nodeAuthProtobuf := &protobuffs.AsyncTask{
		Id:          nd.Id,
		RequestId:   nd.RequestId,
		Input:       nd.Input,
		Output:      nd.Output,
		Service:     nd.Service,
		State:       protobuffs.AsyncTaskState(nd.State),
		DateCreated: timestamppb.New(nd.DateCreated),
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal async task data")
	}

	return nodeAuthDate
}

func (nd *AsyncTask) String() string {
	nodeAuthProtobuf := &protobuffs.AsyncTask{
		Id:          nd.Id,
		RequestId:   nd.RequestId,
		Input:       nd.Input,
		Output:      nd.Output,
		Service:     nd.Service,
		State:       protobuffs.AsyncTaskState(nd.State),
		DateCreated: timestamppb.New(nd.DateCreated),
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal async task data")
	}

	return string(nodeAuthDate)
}

func (nd *AsyncTask) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, AsyncTaskPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	nodeAuthProtobuf := &protobuffs.AsyncTask{
		Id:          nd.Id,
		RequestId:   nd.RequestId,
		Input:       nd.Input,
		Output:      nd.Output,
		Service:     nd.Service,
		State:       protobuffs.AsyncTaskState(nd.State),
		DateCreated: timestamppb.New(nd.DateCreated),
	}

	nodeAuthDate, err := proto.Marshal(nodeAuthProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal async task data")
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(nodeAuthDate)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(nodeAuthDate)

	return n + int64(o), err
}

func (nd *AsyncTask) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != AsyncTaskPayload {
		return n, errors.New("invalid async task payload")
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
	nodeAuthProtobuf := &protobuffs.AsyncTask{}
	err = proto.Unmarshal(buf, nodeAuthProtobuf)
	if err != nil {
		return n, err
	}

	nd.Id = nodeAuthProtobuf.Id
	nd.RequestId = nodeAuthProtobuf.RequestId
	nd.Input = nodeAuthProtobuf.Input
	nd.Output = nodeAuthProtobuf.Output
	nd.Service = nodeAuthProtobuf.Service
	nd.State = AsyncTaskState(protobuffs.AsyncTaskState(nodeAuthProtobuf.State))
	nd.DateCreated = nodeAuthProtobuf.DateCreated.AsTime()

	return n + int64(o), nil
}
