package models

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"scheduler0/pkg/protobuffs"

	"google.golang.org/protobuf/proto"
)

// JobUpdateRequest is a request from a worker node to update a job on the leader
type JobUpdateRequest struct {
	AuthUsername string
	AuthPassword string
	Job          Job
}

func (jur *JobUpdateRequest) Bytes() []byte {
	// Serialize job to JSON
	jobJSON, err := json.Marshal(jur.Job)
	if err != nil {
		log.Fatalln("failed to marshal job to JSON")
	}

	jobUpdateRequestProtobuf := &protobuffs.JobUpdateRequestPayload{
		AuthUsername: jur.AuthUsername,
		AuthPassword: jur.AuthPassword,
		JobData:      jobJSON,
	}

	jobUpdateRequest, err := proto.Marshal(jobUpdateRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal job update request data")
	}

	return jobUpdateRequest
}

func (jur *JobUpdateRequest) String() string {
	// Serialize job to JSON
	jobJSON, err := json.Marshal(jur.Job)
	if err != nil {
		log.Fatalln("failed to marshal job to JSON")
	}

	jobUpdateRequestProtobuf := &protobuffs.JobUpdateRequestPayload{
		AuthUsername: jur.AuthUsername,
		AuthPassword: jur.AuthPassword,
		JobData:      jobJSON,
	}

	jobUpdateRequest, err := proto.Marshal(jobUpdateRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal job update request data")
	}

	return string(jobUpdateRequest)
}

func (jur *JobUpdateRequest) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, JobUpdateRequestPayload)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	// Serialize job to JSON
	jobJSON, err := json.Marshal(jur.Job)
	if err != nil {
		log.Fatalln("failed to marshal job to JSON")
		return n, err
	}

	jobUpdateRequestProtobuf := &protobuffs.JobUpdateRequestPayload{
		AuthUsername: jur.AuthUsername,
		AuthPassword: jur.AuthPassword,
		JobData:      jobJSON,
	}

	jobUpdateRequest, err := proto.Marshal(jobUpdateRequestProtobuf)
	if err != nil {
		log.Fatalln("failed to protobuf marshal job update request data")
		return n, err
	}
	err = binary.Write(w, binary.BigEndian, uint32(len(jobUpdateRequest)))
	if err != nil {
		return n, err
	}
	n += 4
	o, err := w.Write(jobUpdateRequest)

	return n + int64(o), err
}

func (jur *JobUpdateRequest) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != JobUpdateRequestPayload {
		return n, errors.New("invalid JobUpdateRequest payload")
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
	jobUpdateRequestProtobuf := &protobuffs.JobUpdateRequestPayload{}
	err = proto.Unmarshal(buf, jobUpdateRequestProtobuf)
	if err != nil {
		return n, err
	}

	jur.AuthPassword = jobUpdateRequestProtobuf.AuthPassword
	jur.AuthUsername = jobUpdateRequestProtobuf.AuthUsername

	// Deserialize job from JSON
	err = json.Unmarshal(jobUpdateRequestProtobuf.JobData, &jur.Job)
	if err != nil {
		return n, err
	}

	return n + int64(o), nil
}

