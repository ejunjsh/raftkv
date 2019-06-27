package snap

//import (
//	"errors"
//	"fmt"
//	"github.com/dustin/go-humanize"
//	"github.com/ejunjsh/raftkv/pkg/fileutil"
//	"go.uber.org/zap"
//	"io"
//	"io/ioutil"
//	"os"
//	"path/filepath"
//)
//
//var ErrNoDBSnapshot = errors.New("snap: snapshot file doesn't exist")
//
//// SaveDBFrom saves snapshot of the database from the given reader. It
//// guarantees the save operation is atomic.
//func (s *Snapshotter) SaveDBFrom(r io.Reader, id uint64) (int64, error) {
//	f, err := ioutil.TempFile(s.dir, "tmp")
//	if err != nil {
//		return 0, err
//	}
//	var n int64
//	n, err = io.Copy(f, r)
//	if err == nil {
//		err = fileutil.Fsync(f)
//	}
//	f.Close()
//	if err != nil {
//		os.Remove(f.Name())
//		return n, err
//	}
//	fn := s.dbFilePath(id)
//	if fileutil.Exist(fn) {
//		os.Remove(f.Name())
//		return n, nil
//	}
//	err = os.Rename(f.Name(), fn)
//	if err != nil {
//		os.Remove(f.Name())
//		return n, err
//	}
//
//	s.lg.Info(
//		"saved database snapshot to disk",
//		zap.String("path", fn),
//		zap.Int64("bytes", n),
//		zap.String("size", humanize.Bytes(uint64(n))),
//	)
//
//	return n, nil
//}
//
//// DBFilePath returns the file path for the snapshot of the database with
//// given id. If the snapshot does not exist, it returns error.
//func (s *Snapshotter) DBFilePath(id uint64) (string, error) {
//	if _, err := fileutil.ReadDir(s.dir); err != nil {
//		return "", err
//	}
//	fn := s.dbFilePath(id)
//	if fileutil.Exist(fn) {
//		return fn, nil
//	}
//	if s.lg != nil {
//		s.lg.Warn(
//			"failed to find [SNAPSHOT-INDEX].snap.db",
//			zap.Uint64("snapshot-index", id),
//			zap.String("snapshot-file-path", fn),
//			zap.Error(ErrNoDBSnapshot),
//		)
//	}
//	return "", ErrNoDBSnapshot
//}
//
//func (s *Snapshotter) dbFilePath(id uint64) string {
//	return filepath.Join(s.dir, fmt.Sprintf("%016x.snap.db", id))
//}
