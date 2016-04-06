package battery

// #cgo CFLAGS: -std=c99
// #cgo LDFLAGS: -lsetupapi
// #define INITGUID
// #include <stdio.h>
// #include <windows.h>
// #include <ddk/batclass.h>
// #include <setupapi.h>
// #include <devguid.h>
// #include <string.h>
//
// int GetBatteryState(void* ret) {
//   HDEVINFO hdev = SetupDiGetClassDevs(&GUID_DEVICE_BATTERY, 0, 0, DIGCF_PRESENT | DIGCF_DEVICEINTERFACE);
//   if (INVALID_HANDLE_VALUE != hdev) {
//     // Limit search to 100 batteries max
//     for (int idev = 0; idev < 100; idev++) {
//       SP_DEVICE_INTERFACE_DATA did = {0};
//       did.cbSize = sizeof(did);
//
//       if (SetupDiEnumDeviceInterfaces(hdev, 0, &GUID_DEVICE_BATTERY, idev, &did)) {
//         DWORD cbRequired = 0;
//
//         SetupDiGetDeviceInterfaceDetail(hdev, &did, 0, 0, &cbRequired, 0);
//         if (ERROR_INSUFFICIENT_BUFFER == GetLastError()) {
//           PSP_DEVICE_INTERFACE_DETAIL_DATA pdidd = (PSP_DEVICE_INTERFACE_DETAIL_DATA)LocalAlloc(LPTR, cbRequired);
//           if (pdidd) {
//             pdidd->cbSize = sizeof(*pdidd);
//             if (SetupDiGetDeviceInterfaceDetail(hdev, &did, pdidd, cbRequired, &cbRequired, 0)) {
//               // Enumerated a battery.  Ask it for information.
//               memcpy(ret, pdidd->DevicePath, strlen(pdidd->DevicePath));
//               LocalFree(pdidd);
// printf("%lu,%ld\n",BATTERY_UNKNOWN_CAPACITY,BATTERY_UNKNOWN_RATE);
// printf("Capabilities:%d,%d\n",BATTERY_SYSTEM_BATTERY,BATTERY_IS_SHORT_TERM);
//               return strlen(pdidd->DevicePath);
//            }
//          }
//        }
//        else if (ERROR_NO_MORE_ITEMS == GetLastError()) {
//          break;  // Enumeration failed - perhaps we're out of items
//        }
//      }
//     SetupDiDestroyDeviceInfoList(hdev);
//    }
// }
// }
import "C"
import (
	"fmt"
	"math"
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

func get(idx int) (*Battery, error) {
	var ret [255]byte
	l := C.GetBatteryState(unsafe.Pointer(&ret))
	devicePathStr := string(ret[:l])
	devicePath, err := windows.UTF16PtrFromString(devicePathStr)
	if err != nil {
		return nil, FatalError{Err: err}
	}

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
	b, _ := get(0)
	return []*Battery{b}, nil
}
