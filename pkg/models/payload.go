package models

import (
	"errors"
	"io"
)

const (
	NodeAuthPayload uint8 = iota + 1
	StringPayload
	FetchRemoteDataPayload
	AsyncTaskPayload
	QuotaAllocationPayload
	LocalQuotaRequestPayload
	LocalQuotaResponsePayload
	AccountExhaustionPayload
	JobUpdateRequestPayload
	MaxPayloadSize uint32 = 10 << 20 // 10 MB
)

var ErrMaxPayloadSize = errors.New("maximum payload size exceeded")

type NodeTCPPayload interface {
	Bytes() []byte
	String() string
	WriteTo(w io.Writer) (int64, error)
	ReadFrom(r io.Reader) (int64, error)
}
