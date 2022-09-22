package marsher

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io/ioutil"
	protobufs "scheduler0/protobuffs"
)

func MarshalCommand(c *protobufs.Command) ([]byte, error) {
	return proto.Marshal(c)
}

func UnmarshalCommand(b []byte, c *protobufs.Command) error {
	return proto.Unmarshal(b, c)
}

// gzCompress compresses the given byte slice.
func gzCompress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("gzip new writer: %s", err)
	}

	if _, err := gzw.Write(b); err != nil {
		return nil, fmt.Errorf("gzip Write: %s", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("gzip Close: %s", err)
	}
	return buf.Bytes(), nil
}

func gzUncompress(b []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("unmarshal gzip NewReader: %s", err)
	}

	ub, err := ioutil.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("unmarshal gzip ReadAll: %s", err)
	}

	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("unmarshal gzip Close: %s", err)
	}
	return ub, nil
}
