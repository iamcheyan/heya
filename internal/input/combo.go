package input

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

const (
	evKey     = 0x01
	btnSelect = 0x13a // 314
	btnStart  = 0x13b // 315
)

// StartExitComboWatcher listens to /dev/input/event* devices for simultaneous
// press of START and SELECT buttons. When both are held for the debounce duration,
// onCombo is called. Returns a cleanup function that stops all monitoring.
func StartExitComboWatcher(onCombo func()) func() {
	devicePaths, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(devicePaths) == 0 {
		return func() {}
	}

	var files []*os.File
	for _, p := range devicePaths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		files = append(files, f)
	}

	if len(files) == 0 {
		return func() {}
	}

	return StartExitComboWatcherFromFiles(files, 350*time.Millisecond, onCombo)
}

// StartExitComboWatcherFromFiles monitors the provided input event files for the
// START + SELECT combo.
func StartExitComboWatcherFromFiles(files []*os.File, holdDuration time.Duration, onCombo func()) func() {
	if len(files) == 0 {
		return func() {}
	}

	var (
		mu          sync.Mutex
		done        = make(chan struct{})
		wg          sync.WaitGroup
		startCount  int
		selectCount int
		comboTimer  *time.Timer
		triggered   bool
		is64Bit     = unsafe.Sizeof(uintptr(0)) == 8
		eventSize   = 24
	)
	if !is64Bit {
		eventSize = 16
	}

	for _, f := range files {
		wg.Add(1)
		go func(file *os.File) {
			defer wg.Done()
			buf := make([]byte, eventSize)
			var devStart, devSelect bool

			defer func() {
				mu.Lock()
				if devStart {
					startCount--
				}
				if devSelect {
					selectCount--
				}
				mu.Unlock()
			}()

			for {
				select {
				case <-done:
					return
				default:
				}

				_, err := io.ReadFull(file, buf)
				if err != nil {
					return
				}

				var typ, code uint16
				var val int32
				if is64Bit {
					typ = binary.LittleEndian.Uint16(buf[16:18])
					code = binary.LittleEndian.Uint16(buf[18:20])
					val = int32(binary.LittleEndian.Uint32(buf[20:24]))
				} else {
					typ = binary.LittleEndian.Uint16(buf[8:10])
					code = binary.LittleEndian.Uint16(buf[10:12])
					val = int32(binary.LittleEndian.Uint32(buf[12:16]))
				}

				if typ != evKey {
					continue
				}

				mu.Lock()
				switch code {
				case btnStart:
					pressed := (val > 0)
					if pressed != devStart {
						devStart = pressed
						if pressed {
							startCount++
						} else {
							startCount--
						}
					}
				case btnSelect:
					pressed := (val > 0)
					if pressed != devSelect {
						devSelect = pressed
						if pressed {
							selectCount++
						} else {
							selectCount--
						}
					}
				}

				if startCount > 0 && selectCount > 0 {
					if !triggered && comboTimer == nil {
						comboTimer = time.AfterFunc(holdDuration, func() {
							mu.Lock()
							defer mu.Unlock()
							if startCount > 0 && selectCount > 0 && !triggered {
								triggered = true
								if onCombo != nil {
									go onCombo()
								}
							}
						})
					}
				} else {
					triggered = false
					if comboTimer != nil {
						comboTimer.Stop()
						comboTimer = nil
					}
				}
				mu.Unlock()
			}
		}(f)
	}

	return func() {
		close(done)
		mu.Lock()
		if comboTimer != nil {
			comboTimer.Stop()
			comboTimer = nil
		}
		mu.Unlock()
		for _, f := range files {
			_ = f.Close()
		}
		wg.Wait()
	}
}
