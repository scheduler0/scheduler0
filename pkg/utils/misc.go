package utils

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"path"
	"runtime"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/service/etcd"
	"unsafe"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/segmentio/ksuid"
	"github.com/spf13/afero"
)

func MakeDirIfNotExist(path string) (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal("Fatal error getting working dir: ", err.Error())
	}

	dirPath := fmt.Sprintf("%v/%v", dir, path)
	fs := afero.NewOsFs()

	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		return dirPath, exists
	}

	if !exists {
		err := fs.Mkdir(dirPath, os.ModePerm)
		if err != nil {
			log.Fatal("err", err)
			return dirPath, exists
		}
	}

	return dirPath, exists
}

func GetRandomSha256() string {
	randomId := ksuid.New().String()
	hash := sha256.New()
	hash.Write([]byte(randomId))
	return hex.EncodeToString(hash.Sum(nil))
}

func ReadUint64(b []byte) (uint64, error) {
	var sz uint64
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &sz); err != nil {
		return 0, err
	}
	return sz, nil
}

func WriteUint64(w io.Writer, v uint64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func BytesFromSnapshot(rc io.ReadCloser) ([]byte, error) {
	var uint64Size uint64
	inc := int64(unsafe.Sizeof(uint64Size))

	// Read all the data into RAM, since we have to decode known-length
	// chunks of various forms.
	var offset int64
	b, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("readall: %s", err)
	}

	// Get size of database, checking for compression.
	compressed := false
	sz, err := ReadUint64(b[offset : offset+inc])
	if err != nil {
		return nil, fmt.Errorf("read compression check: %s", err)
	}
	offset = offset + inc

	if sz == math.MaxUint64 {
		compressed = true
		// Database is actually compressed, read actual size next.
		sz, err = ReadUint64(b[offset : offset+inc])
		if err != nil {
			return nil, fmt.Errorf("read compressed size: %s", err)
		}
		offset = offset + inc
	}

	// Now read in the database file data, decompress if necessary, and restore.
	var database []byte
	if sz > 0 {
		if compressed {
			buf := new(bytes.Buffer)
			gz, err := gzip.NewReader(bytes.NewReader(b[offset : offset+int64(sz)]))
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(buf, gz); err != nil {
				return nil, fmt.Errorf("SQLite database decompress: %s", err)
			}

			if err := gz.Close(); err != nil {
				return nil, err
			}
			database = buf.Bytes()
		} else {
			if int64(len(b)) < offset+int64(sz) {
				return nil, errors.New("invalid compressed bytes")
			}
			database = b[offset : offset+int64(sz)]
		}
	} else {
		database = nil
	}
	return database, nil
}

func GetNodeServerAddressWithRaftAddress(raftAddress raft.ServerAddress) string {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()

	if len(configs.EtcdEndpoints) == 0 {
		return ""
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "utils-etcd",
		Level: hclog.LevelFromString("INFO"),
	})
	ctx := context.Background()

	etcdSvc, err := etcd.NewEtcdService(ctx, logger, configProvider)
	if err != nil {
		logger.Error("failed to create etcd service in GetNodeServerAddressWithRaftAddress", "error", err)
		return ""
	}
	defer etcdSvc.Close()

	peers, err := etcdSvc.GetPeers(ctx)
	if err != nil {
		logger.Error("failed to get peers from etcd in GetNodeServerAddressWithRaftAddress", "error", err)
		return ""
	}

	for _, peer := range peers {
		if peer.NodeAddress == string(raftAddress) {
			if peer.ClientAddress != "" {
				return peer.ClientAddress
			}
			if peer.NodeAddress != "" {
				return peer.NodeAddress
			}
		}
	}

	return ""
}

func GetNodeIdWithRaftAddress(raftAddress raft.ServerAddress) (int64, error) {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()

	if len(configs.EtcdEndpoints) == 0 {
		return -1, errors.New("etcd endpoints not configured")
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "utils-etcd",
		Level: hclog.LevelFromString("INFO"),
	})
	ctx := context.Background()

	etcdSvc, err := etcd.NewEtcdService(ctx, logger, configProvider)
	if err != nil {
		return -1, fmt.Errorf("failed to create etcd service: %w", err)
	}
	defer etcdSvc.Close()

	peers, err := etcdSvc.GetPeers(ctx)
	if err != nil {
		return -1, fmt.Errorf("failed to get peers from etcd: %w", err)
	}

	for _, peer := range peers {
		if peer.NodeAddress == string(raftAddress) {
			return int64(peer.NodeId), nil
		}
	}

	return -1, errors.New("cannot find node with raft address in etcd")
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func MonitorMemoryUsage(logger hclog.Logger) bool {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if bToMb(m.TotalAlloc) >= configs.MaxMemory {
		logger.Error("max memory reached cannot schedule jobs ", configs.MaxMemory, " MB, total allocated money", bToMb(m.TotalAlloc), "MB")
		return true
	}

	return false
}

func GzCompress(b []byte) ([]byte, error) {
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

func GzUncompress(b []byte) ([]byte, error) {
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

func GetServerHTTPAddress() string {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()
	protocol := "http"
	if configs.HTTPCert != "" {
		protocol = "https"
	}
	return fmt.Sprintf("%v://%v:%v", protocol, configs.Host, configs.ClientPort)
}

func GetNodeAddress() string {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()
	return fmt.Sprintf("%s:%s", configs.Host, configs.NodePort)
}

func GetNodeServiceDiscoveryAddress() string {
	configProvider := config.NewScheduler0Config()
	configs := configProvider.GetConfigurations()
	return configs.NodeServiceDiscoveryAddress
}

func ExpandIdsRange[T uint64 | int64](min, max T) []T {
	results := make([]T, 0, max-min)
	for i := min; i <= max; i++ {
		results = append(results, i)
	}
	return results
}

func GetSqliteDbDirAndDbFilePath() (string, string) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%s/%s", dir, constants.SqliteDir)
	filePath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)

	return dirPath, filePath
}

// GetSqliteDbDirAndDbFilePathForNode returns node-specific SQLite directory and file paths
func GetSqliteDbDirAndDbFilePathForNode(nodeId uint64) (string, string) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%s/%s/%d", dir, constants.SqliteDir, nodeId)
	filePath := fmt.Sprintf("%s/%s/%d/%s", dir, constants.SqliteDir, nodeId, constants.SqliteDbFileName)

	return dirPath, filePath
}

func RemoveSqliteDbDir() {
	dirPath, filePath := GetSqliteDbDirAndDbFilePath()
	fs := afero.NewOsFs()
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}
	if exists {
		removeErr := fs.RemoveAll(dirPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
		removeErr = fs.RemoveAll(filePath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
	}
}

func RemoveRaftDir() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	fs := afero.NewOsFs()

	dirPath := fmt.Sprintf("%v/%v", dir, constants.RaftDir)
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}
	if exists {
		removeErr := fs.RemoveAll(dirPath)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Fatalln(fmt.Errorf("Fatal failed to remove raft dir: %s \n", err))
		}
	}
}

func GetBinPath() string {
	e, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get path of scheduler0 binary", err.Error())
	}
	return path.Dir(e)
}

func Contains(slice []uint64, item uint64) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// FirstPrivateIP returns the first non-loopback private IPv4 address
func FirstPrivateIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			// private ranges
			if ip[0] == 10 || (ip[0] == 172 && ip[1]&0xf0 == 16) || (ip[0] == 192 && ip[1] == 168) {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no private IP found")
}
