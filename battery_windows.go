package battery

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type batteryQueryInformation struct {
	BatteryTag       uint
	InformationLevel int
	AtRate           int
}

type batteryInformation struct {
	Capabilities        uint
	Technology          uint8
	Reserved            [3]uint8
	Chemistry           [4]uint8
	DesignedCapacity    uint
	FullChargedCapacity uint
	DefaultAlert1       uint
	DefaultAlert2       uint
	CriticalBias        uint
	CycleCount          uint
}

type batteryWaitStatus struct {
	BatteryTag   uint
	Timeout      uint
	PowerState   uint
	LowCapacity  uint
	HighCapacity uint
}

type batteryStatus struct {
	PowerState uint
	Capacity   uint
	Voltage    uint
	Rate       int
}

type guid struct {
	Data1 uint
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var guidDeviceBattery = guid{
	0x72631e54,
	0x78A4,
	0x11d0,
	[8]byte{0xbc, 0xf7, 0x00, 0xaa, 0x00, 0xb7, 0xb3, 0x2a},
}

type spDeviceInterfaceData struct {
	cbSize             uint
	InterfaceClassGuid guid
	Flags              uint
	Reserved           uint
}

func intToFloat64(num int) (float64, error) {
	// TODO: Check that this works on 64-bit systems.
	// There is generally something wrong with this constant.
	if num == -0x80000000 { // BATTERY_UNKNOWN_RATE
		return 0, fmt.Errorf("Unknown value received")
	}
	return math.Abs(float64(num)), nil
}

func uintToFloat64(num uint) (float64, error) {
	if num == 0xffffffff { // BATTERY_UNKNOWN_CAPACITY
		return 0, fmt.Errorf("Unknown value received")
	}
	return float64(num), nil
}

func setupDiSetup(proc *windows.LazyProc, nargs, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, error) {
	r1, _, errno := syscall.Syscall6(proc.Addr(), nargs, a1, a2, a3, a4, a5, a6)
	if windows.Handle(r1) == windows.InvalidHandle {
		if errno != 0 {
			return 0, error(errno)
		}
		return 0, syscall.EINVAL
	}
	return r1, nil
}

func setupDiCall(proc *windows.LazyProc, nargs, a1, a2, a3, a4, a5, a6 uintptr) syscall.Errno {
	r1, _, errno := syscall.Syscall6(proc.Addr(), nargs, a1, a2, a3, a4, a5, a6)
	if r1 == 0 { // FIXME: Should use windows.InvalidHandle here
		if errno != 0 {
			return errno
		}
		return syscall.EINVAL
	}
	return 0
}

var setupapi = &windows.LazyDLL{Name: "setupapi.dll", System: true}
var setupDiGetClassDevsW = setupapi.NewProc("SetupDiGetClassDevsW")
var setupDiEnumDeviceInterfaces = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
var setupDiGetDeviceInterfaceDetailW = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
var setupDiDestroyDeviceInfoList = setupapi.NewProc("SetupDiDestroyDeviceInfoList")

func get(idx int) (*Battery, error) {
	hdev, err := setupDiSetup(
		setupDiGetClassDevsW,
		4,
		uintptr(unsafe.Pointer(&guidDeviceBattery)),
		0,
		0,
		2|16, // DIGCF_PRESENT|DIGCF_DEVICEINTERFACE
		0, 0,
	)
	if err != nil {
		return nil, FatalError{Err: err}
	}
	defer syscall.Syscall(setupDiDestroyDeviceInfoList.Addr(), 1, hdev, 0, 0)

	var did spDeviceInterfaceData
	did.cbSize = uint(unsafe.Sizeof(did))
	errno := setupDiCall(
		setupDiEnumDeviceInterfaces,
		5,
		hdev,
		0,
		uintptr(unsafe.Pointer(&guidDeviceBattery)),
		uintptr(idx),
		uintptr(unsafe.Pointer(&did)),
		0,
	)
	if errno != 0 {
		return nil, FatalError{Err: errno}
	}
	var cbRequired uint
	errno = setupDiCall(
		setupDiGetDeviceInterfaceDetailW,
		6,
		hdev,
		uintptr(unsafe.Pointer(&did)),
		0,
		0,
		uintptr(unsafe.Pointer(&cbRequired)),
		0,
	)
	if errno == 259 { //ERROR_NO_MORE_ITEMS
		return nil, FatalError{Err: fmt.Errorf("Not found")} // TODO: Refactor this into typed error
	}
	if errno != 0 && errno != 122 { // ERROR_INSUFFICIENT_BUFFER
		return nil, FatalError{Err: errno}
	}
	// The god damn struct with ANYSIZE_ARRAY of utf16 in it is crazy.
	// So... let's emulate it with array of uint16 ;-D.
	// Keep in mind that the first two/four elements are actually cbSize.
	uintSize := uint(unsafe.Sizeof(uint(0)) / 2)
	didd := make([]uint16, cbRequired/uintSize-1)
	cbSize := (*uint)(unsafe.Pointer(&didd[0]))
	*cbSize = uintSize*2 + 2
	errno = setupDiCall(
		setupDiGetDeviceInterfaceDetailW,
		6,
		hdev,
		uintptr(unsafe.Pointer(&did)),
		uintptr(unsafe.Pointer(&didd[0])),
		uintptr(cbRequired),
		uintptr(unsafe.Pointer(&cbRequired)),
		0,
	)
	if errno != 0 {
		return nil, FatalError{Err: err}
	}
	devicePath := &didd[uintSize:][0]

	handle, err := windows.CreateFile(
		devicePath,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, FatalError{Err: err}
	}
	defer windows.CloseHandle(handle)

	var dwOut uint32

	var dwWait uint32
	var bqi batteryQueryInformation
	err = windows.DeviceIoControl(
		handle,
		2703424, // IOCTL_BATTERY_QUERY_TAG
		(*byte)(unsafe.Pointer(&dwWait)),
		uint32(unsafe.Sizeof(dwWait)),
		(*byte)(unsafe.Pointer(&bqi.BatteryTag)),
		uint32(unsafe.Sizeof(bqi.BatteryTag)),
		&dwOut,
		nil,
	)
	if err != nil {
		return nil, FatalError{Err: err}
	}
	if bqi.BatteryTag == 0 {
		return nil, FatalError{Err: fmt.Errorf("BatteryTag not returned")}
	}

	b := &Battery{Name: fmt.Sprintf("BAT%d", bqi.BatteryTag)}
	e := PartialError{}

	var bi batteryInformation
	err = windows.DeviceIoControl(
		handle,
		2703428, // IOCTL_BATTERY_QUERY_INFORMATION
		(*byte)(unsafe.Pointer(&bqi)),
		uint32(unsafe.Sizeof(bqi)),
		(*byte)(unsafe.Pointer(&bi)),
		uint32(unsafe.Sizeof(bi)),
		&dwOut,
		nil,
	)
	if err == nil {
		b.Full = float64(bi.FullChargedCapacity)
		b.Design = float64(bi.DesignedCapacity)
	} else {
		e.Full = err
		e.Design = err
	}

	bws := batteryWaitStatus{BatteryTag: bqi.BatteryTag}
	var bs batteryStatus
	err = windows.DeviceIoControl(
		handle,
		2703436, // IOCTL_BATTERY_QUERY_STATUS
		(*byte)(unsafe.Pointer(&bws)),
		uint32(unsafe.Sizeof(bws)),
		(*byte)(unsafe.Pointer(&bs)),
		uint32(unsafe.Sizeof(bs)),
		&dwOut,
		nil,
	)
	if err == nil {
		b.Current, e.Current = uintToFloat64(bs.Capacity)
		b.ChargeRate, e.ChargeRate = intToFloat64(bs.Rate)
		switch bs.PowerState {
		case 0x00000004:
			b.State, _ = newState("Charging")
		case 0x00000008:
			b.State, _ = newState("Empty")
		case 0x00000002:
			b.State, _ = newState("Discharging")
		case 0x00000001:
			b.State, _ = newState("Full")
		default:
			b.State, _ = newState("Unknown")
		}
	} else {
		e.Current = err
		e.ChargeRate = err
		e.State = err
	}

	if e.Nil() {
		return b, nil
	}
	return b, e
}

func getAll() ([]*Battery, error) {
	b, e := get(0)
	return []*Battery{b}, e
}
