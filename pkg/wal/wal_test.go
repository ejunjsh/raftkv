package wal

import (
	"bytes"
	"github.com/ejunjsh/kv/pkg/pbutil"
	"github.com/ejunjsh/kv/pkg/raft/raftpb"
	"github.com/ejunjsh/kv/pkg/wal/walpb"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, []byte("somedata"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != walName(0, 0) {
		t.Errorf("name = %+v, want %+v", g, walName(0, 0))
	}
	defer w.Close()

	// file is preallocated to segment size; only read data written by wal
	off, err := w.tail().Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	gd := make([]byte, off)
	f, err := os.Open(filepath.Join(p, filepath.Base(w.tail().Name())))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = io.ReadFull(f, gd); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	var wb bytes.Buffer
	e := newEncoder(&wb, 0, 0)
	err = e.encode(&walpb.Record{Type: crcType, Crc: 0})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	err = e.encode(&walpb.Record{Type: metadataType, Data: []byte("somedata")})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	r := &walpb.Record{
		Type: snapshotType,
		Data: pbutil.MustMarshal(&walpb.Snapshot{}),
	}
	if err = e.encode(r); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	e.flush()
	if !bytes.Equal(gd, wb.Bytes()) {
		t.Errorf("data = %v, want %v", gd, wb.Bytes())
	}
}

func TestCreateFailFromPollutedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	ioutil.WriteFile(filepath.Join(p, "test.wal"), []byte("data"), os.ModeTemporary)

	_, err = Create(zap.NewExample(), p, []byte("data"))
	if err != os.ErrExist {
		t.Fatalf("expected %v, got %v", os.ErrExist, err)
	}
}

func TestCreateFailFromNoSpaceLeft(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	oldSegmentSizeBytes := SegmentSizeBytes
	defer func() {
		SegmentSizeBytes = oldSegmentSizeBytes
	}()
	SegmentSizeBytes = math.MaxInt64

	_, err = Create(zap.NewExample(), p, []byte("data"))
	if err == nil { // no space left on device
		t.Fatalf("expected error 'no space left on device', got nil")
	}
}

func TestNewForInitedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	os.Create(filepath.Join(p, walName(0, 0)))
	if _, err = Create(zap.NewExample(), p, nil); err == nil || err != os.ErrExist {
		t.Errorf("err = %v, want %v", err, os.ErrExist)
	}
}

func TestOpenAtIndex(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	f, err := os.Create(filepath.Join(dir, walName(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err := Open(zap.NewExample(), dir, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != walName(0, 0) {
		t.Errorf("name = %+v, want %+v", g, walName(0, 0))
	}
	if w.seq() != 0 {
		t.Errorf("seq = %d, want %d", w.seq(), 0)
	}
	w.Close()

	wname := walName(2, 10)
	f, err = os.Create(filepath.Join(dir, wname))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err = Open(zap.NewExample(), dir, walpb.Snapshot{Index: 5})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != wname {
		t.Errorf("name = %+v, want %+v", g, wname)
	}
	if w.seq() != 2 {
		t.Errorf("seq = %d, want %d", w.seq(), 2)
	}
	w.Close()

	emptydir, err := ioutil.TempDir(os.TempDir(), "waltestempty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emptydir)
	if _, err = Open(zap.NewExample(), emptydir, walpb.Snapshot{}); err != ErrFileNotFound {
		t.Errorf("err = %v, want %v", err, ErrFileNotFound)
	}
}

// TestVerify tests that Verify throws a non-nil error when the WAL is corrupted.
// The test creates a WAL directory and cuts out multiple WAL files. Then
// it corrupts one of the files by completely truncating it.
func TestVerify(t *testing.T) {
	walDir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(walDir)

	// create WAL
	w, err := Create(zap.NewExample(), walDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// make 5 separate files
	for i := 0; i < 5; i++ {
		es := []raftpb.Entry{{Index: uint64(i), Data: []byte("waldata" + string(i+1))}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = w.cut(); err != nil {
			t.Fatal(err)
		}
	}

	// to verify the WAL is not corrupted at this point
	err = Verify(zap.NewExample(), walDir, walpb.Snapshot{})
	if err != nil {
		t.Errorf("expected a nil error, got %v", err)
	}

	walFiles, err := ioutil.ReadDir(walDir)
	if err != nil {
		t.Fatal(err)
	}

	// corrupt the WAL by truncating one of the WAL files completely
	err = os.Truncate(path.Join(walDir, walFiles[2].Name()), 0)
	if err != nil {
		t.Fatal(err)
	}

	err = Verify(zap.NewExample(), walDir, walpb.Snapshot{})
	if err == nil {
		t.Error("expected a non-nil error, got nil")
	}
}
