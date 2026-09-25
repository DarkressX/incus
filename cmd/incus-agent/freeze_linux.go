//go:build linux

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/lxc/incus/v7/shared/logger"
	"github.com/lxc/incus/v7/shared/subprocess"
)

// freezeFreezableFilesystems lists the filesystems which are frozen during snapshots. Only
// filesystems known to support freezing are included so that freezing one that does not would fail
// the whole snapshot.
var freezeFreezableFilesystems = []string{
	"btrfs", "ext2", "ext3", "ext4", "f2fs", "jfs", "nilfs2", "reiserfs", "xfs",
}

// Freeze state so that only the filesystems frozen here get thawed later.
var freezeMu sync.Mutex
var freezeFrozen []string

// osFreezeFilesystems freezes all freezable filesystems and returns the frozen mount points.
func osFreezeFilesystems() ([]string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("Failed to read /proc/self/mountinfo: %w", err)
	}

	mountPoints := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()

		// Split the mount point and its options from the filesystem type and source.
		sepIdx := strings.Index(line, " - ")
		if sepIdx < 0 {
			continue
		}

		left := strings.Fields(line[:sepIdx])
		right := strings.Fields(line[sepIdx+3:])
		if len(left) < 6 || len(right) < 2 {
			continue
		}

		mountPoint := left[4]
		mountOptions := left[5]
		fstype := right[0]

		// Skip filesystems which cannot be frozen.
		if !slices.Contains(freezeFreezableFilesystems, fstype) {
			continue
		}

		// Skip read-only filesystems as there is nothing to freeze.
		if slices.Contains(strings.Split(mountOptions, ","), "ro") {
			continue
		}

		mountPoints = append(mountPoints, mountPoint)
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	// Freeze nested mount points first so that freezing a parent cannot interfere with accessing
	// a filesystem mounted below it.
	sort.Slice(mountPoints, func(i int, j int) bool {
		return strings.Count(mountPoints[i], "/") > strings.Count(mountPoints[j], "/")
	})

	frozen := []string{}
	for _, mountPoint := range mountPoints {
		_, err = subprocess.RunCommand("fsfreeze", "--freeze", mountPoint)
		if err != nil {
			// Thaw any filesystems frozen so far before reporting the failure.
			for _, path := range frozen {
				_, _ = subprocess.RunCommand("fsfreeze", "--unfreeze", path)
			}

			return nil, fmt.Errorf("Failed freezing filesystem %q: %w", mountPoint, err)
		}

		frozen = append(frozen, mountPoint)
	}

	freezeMu.Lock()
	freezeFrozen = frozen
	freezeMu.Unlock()

	return frozen, nil
}

// osUnfreezeFilesystems thaws all filesystems previously frozen by osFreezeFilesystems.
func osUnfreezeFilesystems() ([]string, error) {
	freezeMu.Lock()
	defer freezeMu.Unlock()

	unfrozen := []string{}
	var firstErr error
	for _, mountPoint := range freezeFrozen {
		_, err := subprocess.RunCommand("fsfreeze", "--unfreeze", mountPoint)
		if err != nil {
			logger.Warn("Failed unfreezing filesystem", logger.Ctx{"path": mountPoint, "err": err})
			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		unfrozen = append(unfrozen, mountPoint)
	}

	freezeFrozen = nil

	return unfrozen, firstErr
}
