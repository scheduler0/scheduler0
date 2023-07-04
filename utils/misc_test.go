package utils

import (
	"bytes"
	"encoding/binary"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestGetRandomSha256(t *testing.T) {
	sha1 := GetRandomSha256()
	sha2 := GetRandomSha256()

	if len(sha1) != 64 {
		t.Errorf("GetRandomSha256() length should be 64, but got %v", len(sha1))
	}

	if len(sha2) != 64 {
		t.Errorf("GetRandomSha256() length should be 64, but got %v", len(sha2))
	}

	if sha1 == sha2 {
		t.Errorf("GetRandomSha256() should generate unique hash values, but got equal values: %v and %v", sha1, sha2)
	}
}

func TestReadUint64(t *testing.T) {
	var testCases = []struct {
		name      string
		input     []byte
		expected  uint64
		expectErr bool
	}{
		{
			name:      "Valid Input",
			input:     []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01},
			expected:  72623859790382856,
			expectErr: false,
		},
		{
			name:      "Invalid Input",
			input:     []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02},
			expected:  0,
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := ReadUint64(testCase.input)

			if testCase.expectErr && err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !testCase.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result != testCase.expected {
				t.Errorf("Expected result %v, but got %v", testCase.expected, result)
			}
		})
	}
}

func TestWriteUint64(t *testing.T) {
	var testCases = []struct {
		name     string
		input    uint64
		expected []byte
	}{
		{
			name:     "Valid Input",
			input:    72623859790382856,
			expected: []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteUint64(&buf, testCase.input)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			result := buf.Bytes()

			if !bytes.Equal(result, testCase.expected) {
				t.Errorf("Expected result %v, but got %v", testCase.expected, result)
			}
		})
	}
}

func TestBytesFromSnapshot(t *testing.T) {
	testCases := []struct {
		name             string
		input            []byte
		expectedDatabase []byte
		expectError      bool
	}{
		{
			name:             "Valid Uncompressed Input",
			input:            createSnapshotBytes(false),
			expectedDatabase: []byte("Test Database"),
			expectError:      false,
		},
		{
			name:             "Valid Compressed Input",
			input:            createSnapshotBytes(true),
			expectedDatabase: []byte("Test Database"),
			expectError:      false,
		},
		{
			name:        "Invalid Input",
			input:       []byte("Invalid"),
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rc := ioutil.NopCloser(bytes.NewReader(testCase.input))
			result, err := BytesFromSnapshot(rc)

			if testCase.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if !bytes.Equal(result, testCase.expectedDatabase) {
					t.Errorf("Expected result %v, but got %v", testCase.expectedDatabase, result)
				}
			}
		})
	}
}

func createSnapshotBytes(compressed bool) []byte {
	var buf bytes.Buffer

	if compressed {
		binary.Write(&buf, binary.LittleEndian, uint64(math.MaxUint64))
	}

	db := []byte("Test Database")
	var dbBytes []byte
	var err error

	if compressed {
		dbBytes, err = GzCompress(db)
		if err != nil {
			return nil
		}
	} else {
		dbBytes = db
	}

	binary.Write(&buf, binary.LittleEndian, uint64(len(dbBytes)))
	buf.Write(dbBytes)

	return buf.Bytes()
}

func TestExpandIdsRange(t *testing.T) {
	testCases := []struct {
		name   string
		min    uint64
		max    uint64
		result []uint64
	}{
		{
			name:   "Expand Range 1 to 5",
			min:    1,
			max:    5,
			result: []uint64{1, 2, 3, 4, 5},
		},
		{
			name:   "Expand Range 0 to 0",
			min:    0,
			max:    0,
			result: []uint64{0},
		},
		{
			name:   "Expand Range 5 to 5",
			min:    5,
			max:    5,
			result: []uint64{5},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := ExpandIdsRange(testCase.min, testCase.max)

			if !reflect.DeepEqual(result, testCase.result) {
				t.Errorf("Expected result %v, but got %v", testCase.result, result)
			}
		})
	}
}

func Test_GetNodeServerAddressWithRaftAddress(t *testing.T) {
	os.Setenv("SCHEDULER0_REPLICAS", `[{"nodeId":1, "address":"localhost:1234", "raft_address":"localhost:6789"}]`)
	defer os.Remove("SCHEDULER0_REPLICAS")

	raftServerAddress := raft.ServerAddress("localhost:6789")
	nodeServerAddress := GetNodeServerAddressWithRaftAddress(raftServerAddress)
	assert.Equal(t, nodeServerAddress, "localhost:1234")
}

func Test_GetNodeIdWithRaftAddress(t *testing.T) {
	os.Setenv("SCHEDULER0_REPLICAS", `[{"nodeId":1, "address":"localhost:1234", "raft_address":"localhost:6789"}]`)
	defer os.Remove("SCHEDULER0_REPLICAS")

	nodeId, err := GetNodeIdWithServerAddress("localhost:1234")
	assert.Equal(t, err, nil)
	assert.Equal(t, int64(1), nodeId)
}
