package fsm

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/hashicorp/raft"
	"math"
	"scheduler0/db"
	"scheduler0/utils"
)

type fsmSnapshot struct {
	db db.DataStore
}

func NewFSMSnapshot(db db.DataStore) *fsmSnapshot {
	return &fsmSnapshot{
		db: db,
	}
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	dbSerialized := s.db.Serialize()

	err := func() error {
		b := new(bytes.Buffer)

		// Flag compressed database by writing max uint64 value first.
		// No SQLite database written by earlier versions will have this
		// as a size. *Surely*.
		err := utils.WriteUint64(b, math.MaxUint64)
		if err != nil {
			return err
		}
		if _, err := sink.Write(b.Bytes()); err != nil {
			return err
		}
		b.Reset() // Clear state of buffer for future use.

		var buf bytes.Buffer
		gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)

		if err != nil {
			return err
		}

		_, err = gz.Write(dbSerialized)
		if err != nil {
			return err
		}

		if err := gz.Close(); err != nil {
			return err
		}

		compressedDbBytes := buf.Bytes()

		err = utils.WriteUint64(b, uint64(len(compressedDbBytes)))
		if err != nil {
			return err
		}
		if _, err := sink.Write(b.Bytes()); err != nil {
			return err
		}

		// Write compressed database to sink.
		if _, err := sink.Write(compressedDbBytes); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
		return err
	}

	return nil
}

func (s *fsmSnapshot) Release() {
	fmt.Println("Release::")
}
