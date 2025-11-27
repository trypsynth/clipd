//go:build windows

package clipd

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func ToUTF16Ptr(s string, context string) (*uint16, error) {
	ptr, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %s: %w", context, err)
	}
	return ptr, nil
}

func OptionalUTF16Ptr(s string, context string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	return ToUTF16Ptr(s, context)
}
