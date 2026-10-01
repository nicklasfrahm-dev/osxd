// Package evdev reads keyboards through /dev/input and writes key events to a
// virtual keyboard through /dev/uinput.
//
// Only 64-bit Linux is supported: the event struct layout below assumes a
// 64-bit struct timeval.
package evdev

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Event types and codes, from linux/input-event-codes.h.
const (
	EvSyn uint16 = 0x00
	EvKey uint16 = 0x01
	EvRel uint16 = 0x02
	EvAbs uint16 = 0x03
	EvLed uint16 = 0x11

	synReport = 0
	keyMax    = 0x2ff
	ledMax    = 0x0f
	keySpace  = 57
)

// VirtualName is the name of the virtual keyboard, so it is never grabbed.
const VirtualName = "osxd virtual keyboard"

// Event is a raw input event.
type Event struct {
	Type  uint16
	Code  uint16
	Value int32
}

// eventSize is sizeof(struct input_event) on 64-bit Linux.
const eventSize = 24

func (e Event) marshal() []byte {
	b := make([]byte, eventSize) // the kernel fills in the timestamp
	binary.NativeEndian.PutUint16(b[16:], e.Type)
	binary.NativeEndian.PutUint16(b[18:], e.Code)
	binary.NativeEndian.PutUint32(b[20:], uint32(e.Value))
	return b
}

func unmarshal(b []byte) Event {
	return Event{
		Type:  binary.NativeEndian.Uint16(b[16:]),
		Code:  binary.NativeEndian.Uint16(b[18:]),
		Value: int32(binary.NativeEndian.Uint32(b[20:])),
	}
}

func readEvents(f *os.File) ([]Event, error) {
	buf := make([]byte, eventSize*64)
	n, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	var evs []Event
	for i := 0; i+eventSize <= n; i += eventSize {
		evs = append(evs, unmarshal(buf[i:]))
	}
	return evs, nil
}

// ioctl request numbers, from linux/input.h and linux/uinput.h.
func ioc(dir, typ, nr, size uintptr) uintptr { return dir<<30 | size<<16 | typ<<8 | nr }

const (
	iocWrite = 1
	iocRead  = 2
)

var (
	eviocgrab   = ioc(iocWrite, 'E', 0x90, 4)
	uiSetEvbit  = ioc(iocWrite, 'U', 100, 4)
	uiSetKeybit = ioc(iocWrite, 'U', 101, 4)
	uiSetLedbit = ioc(iocWrite, 'U', 105, 4)
	uiDevSetup  = ioc(iocWrite, 'U', 3, unsafe.Sizeof(uinputSetup{}))
	uiDevCreate = ioc(0, 'U', 1, 0)
)

func eviocgname(n uintptr) uintptr    { return ioc(iocRead, 'E', 0x06, n) }
func eviocgkey(n uintptr) uintptr     { return ioc(iocRead, 'E', 0x18, n) }
func eviocgbit(ev, n uintptr) uintptr { return ioc(iocRead, 'E', 0x20+ev, n) }

func ioctl(fd, req, arg uintptr) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, arg); errno != 0 {
		return errno
	}
	return nil
}

func ioctlBuf(f *os.File, req func(uintptr) uintptr, size int) ([]byte, error) {
	b := make([]byte, size)
	err := ioctl(f.Fd(), req(uintptr(size)), uintptr(unsafe.Pointer(&b[0])))
	return b, err
}

func hasBit(bits []byte, n int) bool { return n/8 < len(bits) && bits[n/8]&(1<<(n%8)) != 0 }

// Keyboard is a grabbed physical keyboard.
type Keyboard struct {
	Path string
	Name string
	f    *os.File
}

// Keyboards lists the event devices under /dev/input that look like
// keyboards. Devices that also move a pointer, such as keyboards with a
// touchpad, are skipped, because grabbing them would freeze the pointer.
func Keyboards() ([]string, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		if isKeyboard(f) {
			out = append(out, p)
		}
		f.Close()
	}
	return out, nil
}

func isKeyboard(f *os.File) bool {
	name, _ := ioctlBuf(f, eviocgname, 256)
	if strings.TrimRight(string(name), "\x00") == VirtualName {
		return false
	}
	evs, err := ioctlBuf(f, func(n uintptr) uintptr { return eviocgbit(0, n) }, 4)
	if err != nil || !hasBit(evs, int(EvKey)) || hasBit(evs, int(EvRel)) || hasBit(evs, int(EvAbs)) {
		return false
	}
	keys, err := ioctlBuf(f, func(n uintptr) uintptr { return eviocgbit(uintptr(EvKey), n) }, keyMax/8+1)
	if err != nil {
		return false
	}
	// A letter row and the space bar: rules out power buttons, headset
	// controls and other devices that only report a few keys.
	for _, k := range []int{16, 30, 44, keySpace} { // Q, A, Z, space
		if !hasBit(keys, k) {
			return false
		}
	}
	return true
}

// Grab opens the keyboard at path and takes it over: its events reach only
// this process until Close. It waits up to a second for all keys to be
// released first, so a key held while grabbing (such as the Enter that
// started osxd) does not stay pressed for the desktop.
func Grab(path string) (*Keyboard, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		// Without write access the LEDs cannot be set, which is fine.
		if f, err = os.Open(path); err != nil {
			return nil, err
		}
	}
	name, _ := ioctlBuf(f, eviocgname, 256)
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		held, err := ioctlBuf(f, eviocgkey, keyMax/8+1)
		if err != nil || !anySet(held) {
			break
		}
	}
	if err := ioctl(f.Fd(), eviocgrab, 1); err != nil {
		f.Close()
		return nil, fmt.Errorf("grab %s: %w", path, err)
	}
	return &Keyboard{Path: path, Name: strings.TrimRight(string(name), "\x00"), f: f}, nil
}

func anySet(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return true
		}
	}
	return false
}

// Read blocks until the keyboard reports events.
func (k *Keyboard) Read() ([]Event, error) { return readEvents(k.f) }

// SetLED switches one of the keyboard's LEDs, such as Caps Lock.
func (k *Keyboard) SetLED(code uint16, on bool) {
	var v int32
	if on {
		v = 1
	}
	_, _ = k.f.Write(append(Event{EvLed, code, v}.marshal(), Event{EvSyn, synReport, 0}.marshal()...))
}

// Close releases the keyboard.
func (k *Keyboard) Close() error { return k.f.Close() }

// Virtual is a virtual keyboard created through /dev/uinput.
type Virtual struct{ f *os.File }

// uinputSetup mirrors struct uinput_setup.
type uinputSetup struct {
	bustype, vendor, product, version uint16
	name                              [80]byte
	ffEffectsMax                      uint32
}

// NewVirtual creates a virtual keyboard that can send any key.
func NewVirtual() (*Virtual, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	fd := f.Fd()
	set := func(req, v uintptr) {
		if err == nil {
			err = ioctl(fd, req, v)
		}
	}
	set(uiSetEvbit, uintptr(EvKey))
	set(uiSetEvbit, uintptr(EvLed))
	for k := 1; k <= keyMax; k++ {
		// Leave out mouse, joystick and gamepad buttons, or libinput may
		// treat the device as something other than a keyboard.
		if k >= 0x100 && k < 0x160 || k >= 0x220 && k < 0x230 || k >= 0x2c0 {
			continue
		}
		set(uiSetKeybit, uintptr(k))
	}
	for l := 0; l <= ledMax; l++ {
		set(uiSetLedbit, uintptr(l))
	}
	setup := uinputSetup{bustype: 0x06 /* BUS_VIRTUAL */, vendor: 0x1d6b, product: 0x0104, version: 1}
	copy(setup.name[:], VirtualName)
	set(uiDevSetup, uintptr(unsafe.Pointer(&setup)))
	set(uiDevCreate, 0)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("create virtual keyboard: %w", err)
	}
	return &Virtual{f: f}, nil
}

// Key presses (1), repeats (2) or releases (0) code on the virtual keyboard.
func (v *Virtual) Key(code uint16, value int32) error {
	_, err := v.f.Write(append(Event{EvKey, code, value}.marshal(), Event{EvSyn, synReport, 0}.marshal()...))
	return err
}

// ReadLEDs blocks until the desktop switches LEDs, such as Caps Lock, on the
// virtual keyboard, and returns those events.
func (v *Virtual) ReadLEDs() ([]Event, error) {
	for {
		evs, err := readEvents(v.f)
		if err != nil {
			return nil, err
		}
		var leds []Event
		for _, e := range evs {
			if e.Type == EvLed {
				leds = append(leds, e)
			}
		}
		if len(leds) > 0 {
			return leds, nil
		}
	}
}

// Close removes the virtual keyboard.
func (v *Virtual) Close() error { return v.f.Close() }

// IsGone reports whether err means the device was unplugged.
func IsGone(err error) bool { return errors.Is(err, unix.ENODEV) || errors.Is(err, os.ErrClosed) }
