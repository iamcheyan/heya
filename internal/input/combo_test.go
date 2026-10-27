package input

import (
	"encoding/binary"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func writeEvent(t *testing.T, w *os.File, typ, code uint16, val int32) {
	t.Helper()
	is64Bit := unsafe.Sizeof(uintptr(0)) == 8
	var buf []byte
	if is64Bit {
		buf = make([]byte, 24)
		binary.LittleEndian.PutUint16(buf[16:18], typ)
		binary.LittleEndian.PutUint16(buf[18:20], code)
		binary.LittleEndian.PutUint32(buf[20:24], uint32(val))
	} else {
		buf = make([]byte, 16)
		binary.LittleEndian.PutUint16(buf[8:10], typ)
		binary.LittleEndian.PutUint16(buf[10:12], code)
		binary.LittleEndian.PutUint32(buf[12:16], uint32(val))
	}
	if _, err := w.Write(buf); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}
}

func TestStartExitComboWatcher_Trigger(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var called int32
	stop := StartExitComboWatcherFromFiles([]*os.File{r}, 50*time.Millisecond, func() {
		atomic.AddInt32(&called, 1)
	})
	defer stop()

	// Press START
	writeEvent(t, w, evKey, btnStart, 1)
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&called) != 0 {
		t.Fatal("combo triggered with only START")
	}

	// Press SELECT
	writeEvent(t, w, evKey, btnSelect, 1)

	// Wait for hold duration (50ms) + margin
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected combo to be triggered once, got %d", atomic.LoadInt32(&called))
	}
}

func TestStartExitComboWatcher_ReleaseCancels(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	var called int32
	stop := StartExitComboWatcherFromFiles([]*os.File{r}, 80*time.Millisecond, func() {
		atomic.AddInt32(&called, 1)
	})
	defer stop()

	// Press START and SELECT
	writeEvent(t, w, evKey, btnStart, 1)
	writeEvent(t, w, evKey, btnSelect, 1)

	// Release SELECT after 20ms (before 80ms)
	time.Sleep(20 * time.Millisecond)
	writeEvent(t, w, evKey, btnSelect, 0)

	// Wait past duration
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&called) != 0 {
		t.Fatalf("expected combo to NOT be triggered, got %d", atomic.LoadInt32(&called))
	}
}
