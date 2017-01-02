// +build windows


package ibis

import (
	// "encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	TAPWIN32_MAX_REG_SIZE = 256
	TUNTAP_COMPONENT_ID   = "tap0901"
	ADAPTER_KEY           = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
)

var (
	TAP_IOCTL_GET_MTU          = tap_control_code(3, 0)
	TAP_IOCTL_SET_MEDIA_STATUS = tap_control_code(6, 0)
	TAP_IOCTL_CONFIG_TUN       = tap_control_code(10, 0)
)

type tun struct {
	mtu         int
	device_path string
	fd          syscall.Handle
}

func OpenTunTap() (*tun, error) {
	t := new(tun)
	id, err := getTuntapComponentId()
	if err != nil {
		return nil, err
	}
	t.device_path = fmt.Sprintf(`\\.\Global\%s.tap`, id)
	name := syscall.StringToUTF16(t.device_path)
	tuntap, err := syscall.CreateFile(
		&name[0],
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_SYSTEM|syscall.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		fmt.Println("here")
		return nil, err
	}


	mtu := 1500

	// set connect.
	inBuffer := []byte("\x01\x00\x00\x00")
	if err = syscall.DeviceIoControl(
		tuntap,
		TAP_IOCTL_SET_MEDIA_STATUS,
		&inBuffer[0],
		uint32(len(inBuffer)),
		&inBuffer[0],
		uint32(len(inBuffer)),
		&returnLen,
		nil); err != nil {
		return nil, err
	}
	t.fd = tuntap
	t.mtu = int(mtu)
	return t, nil
}

func getTuntapComponentId() (string, error) {
	adapters, err := registry.OpenKey(registry.LOCAL_MACHINE, ADAPTER_KEY, registry.READ)
	if err != nil {
		return "", err
	}
	var i uint32
	for ; i < 1000; i++ {
		var name_length uint32 = TAPWIN32_MAX_REG_SIZE
		buf := make([]uint16, name_length)
		if err = syscall.RegEnumKeyEx(
			syscall.Handle(adapters),
			i,
			&buf[0],
			&name_length,
			nil,
			nil,
			nil,
			nil); err != nil {
			return "", err
		}
		key_name := syscall.UTF16ToString(buf[:])
		adapter, err := registry.OpenKey(adapters, key_name, registry.READ)
		if err != nil {
			return "", err
		}
		name := syscall.StringToUTF16("ComponentId")
		name2 := syscall.StringToUTF16("NetCfgInstanceId")
		var valtype uint32
		var component_id = make([]byte, TAPWIN32_MAX_REG_SIZE)
		var componentLen = uint32(len(component_id))
		if err = syscall.RegQueryValueEx(
			syscall.Handle(adapter),
			&name[0],
			nil,
			&valtype,
			&component_id[0],
			&componentLen); err != nil {
			return "", err
		}

		if unicodeTostring(component_id) == TUNTAP_COMPONENT_ID {
			var valtype uint32
			var netCfgInstanceId = make([]byte, TAPWIN32_MAX_REG_SIZE)
			var netCfgInstanceIdLen = uint32(len(netCfgInstanceId))
			if err = syscall.RegQueryValueEx(
				syscall.Handle(adapter),
				&name2[0],
				nil,
				&valtype,
				&netCfgInstanceId[0],
				&netCfgInstanceIdLen); err != nil {
				return "", err
			}
			fmt.Println("Device:", unicodeTostring(netCfgInstanceId))
			return unicodeTostring(netCfgInstanceId), nil
		}
	}
	return "", errors.New("not found component id")
}

func unicodeTostring(src []byte) string {
	dst := make([]byte, 0)
	for _, ch := range src {
		if ch != byte(0) {
			dst = append(dst, ch)
		}
	}
	return string(dst)
}

func ctl_code(device_type, function, method, access uint32) uint32 {
	return (device_type << 16) | (access << 14) | (function << 2) | method
}

func tap_control_code(request, method uint32) uint32 {
	return ctl_code(34, request, method, 0)
}
