//go:build windows

package internal

import (
	"encoding/base64"
	"errors"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var procProtect = crypt32.NewProc("CryptProtectData")
var procUnprotect = crypt32.NewProc("CryptUnprotectData")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var procLocalFree = kernel.NewProc("LocalFree")

func cryptBlob(input []byte, decrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("empty payload")
	}
	in := dataBlob{cbData: uint32(len(input)), pbData: &input[0]}
	var out dataBlob
	proc := procProtect
	if decrypt {
		proc = procUnprotect
	}
	var r uintptr
	var err error
	if decrypt {
		r, _, err = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	} else {
		r, _, err = proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	}
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	result := make([]byte, out.cbData)
	copy(result, unsafe.Slice(out.pbData, out.cbData))
	return result, nil
}
func SealMessage(message string) (string, error) {
	raw, err := cryptBlob([]byte(message), false)
	if err != nil {
		return "", err
	}
	return "dpapi:" + base64.StdEncoding.EncodeToString(raw), nil
}
func OpenMessage(cipher string) (string, error) {
	if len(cipher) < 6 || cipher[:6] != "dpapi:" {
		return "", errors.New("알 수 없는 암호화 형식")
	}
	raw, err := base64.StdEncoding.DecodeString(cipher[6:])
	if err != nil {
		return "", err
	}
	plain, err := cryptBlob(raw, true)
	return string(plain), err
}
