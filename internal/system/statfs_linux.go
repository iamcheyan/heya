package system

import (
	"syscall"
	"unsafe"
)

type syscallStatfs struct {
	blockSize uint64
	blocks    uint64
	available uint64
}

func (s syscallStatfs) freeMB() uint64 {
	return s.available * s.blockSize / (1024 * 1024)
}

func (s syscallStatfs) totalMB() uint64 {
	return s.blocks * s.blockSize / (1024 * 1024)
}

func statfs(path string, out *syscallStatfs) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}
	out.blockSize = uint64(stat.Bsize)
	out.blocks = stat.Blocks
	out.available = stat.Bavail
	return nil
}

var _ = unsafe.Pointer(nil)
