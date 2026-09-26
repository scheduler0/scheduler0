package models

import (
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"scheduler0/pkg/protobuffs"
)

type FetchRemoteData struct {
	Phase        string
	AuthUsername string
	AuthPassword string
	RequestId    string
}

func (nd *FetchRemoteData) Bytes() []byte {
	fetchRemoteDataProtobuf := &protobuffs.FetchRemoteDataPayload{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
		Phase:        nd.Phase,
		RequestId:    nd.RequestId,
	}

	fetchRemoteData, err := proto.Marshal(fetchRemoteDataProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal fetch remote data")
	}

	return fetchRemoteData
}

func (nd *FetchRemoteData) String() string {
	fetchRemoteDataProtobuf := &protobuffs.FetchRemoteDataPayload{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
		Phase:        nd.Phase,
		RequestId:    nd.RequestId,
	}

	fetchRemoteData, err := proto.Marshal(fetchRemoteDataProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal fetch remote data")
	}

	return string(fetchRemoteData)
}

func (nd *FetchRemoteData) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, FetchRemoteDataPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	fetchRemoteDataProtobuf := &protobuffs.FetchRemoteDataPayload{
		AuthUsername: nd.AuthUsername,
		AuthPassword: nd.AuthPassword,
		Phase:        nd.Phase,
		RequestId:    nd.RequestId,
	}

	fetchRemoteData, err := proto.Marshal(fetchRemoteDataProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal fetch remote data")
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(fetchRemoteData)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(fetchRemoteData)

	return n + int64(o), err
}

func (nd *FetchRemoteData) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != FetchRemoteDataPayload {
		return n, errors.New("invalid fetch remote data payload")
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
	fetchRemoteDataProtobuf := &protobuffs.FetchRemoteDataPayload{}
	err = proto.Unmarshal(buf, fetchRemoteDataProtobuf)
	if err != nil {
		return n, err
	}

	nd.AuthPassword = fetchRemoteDataProtobuf.AuthPassword
	nd.AuthUsername = fetchRemoteDataProtobuf.AuthUsername
	nd.Phase = fetchRemoteDataProtobuf.Phase
	nd.RequestId = fetchRemoteDataProtobuf.RequestId

	return n + int64(o), nil
}
