//go:build !linux

package main

import "errors"

var errFilesystemFreezeUnsupported = errors.New("Filesystem freezing is not supported on this platform")

func osFreezeFilesystems() ([]string, error) {
	return nil, errFilesystemFreezeUnsupported
}

func osUnfreezeFilesystems() ([]string, error) {
	return nil, errFilesystemFreezeUnsupported
}
