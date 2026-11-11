//go:build !windows

package internal

import "encoding/base64"

func SealMessage(message string) (string, error) {
	return "test-only:" + base64.StdEncoding.EncodeToString([]byte(message)), nil
}
func OpenMessage(cipher string) (string, error) {
	const prefix = "test-only:"
	if len(cipher) < len(prefix) || cipher[:len(prefix)] != prefix {
		return "", base64.CorruptInputError(0)
	}
	raw, err := base64.StdEncoding.DecodeString(cipher[len(prefix):])
	return string(raw), err
}
