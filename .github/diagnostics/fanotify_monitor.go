// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

//go:build linux

// fanotify_monitor records the process responsible for writes below .git.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type event struct {
	Time    string `json:"time"`
	PID     int32  `json:"pid"`
	PPID    string `json:"ppid"`
	Mask    string `json:"mask"`
	Path    string `json:"path"`
	Command string `json:"command"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fanotify_monitor <mount-path>")
		os.Exit(2)
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal(err)
	}
	fd, err := unix.FanotifyInit(unix.FAN_CLASS_NOTIF|unix.FAN_CLOEXEC, unix.O_RDONLY|unix.O_LARGEFILE)
	if err != nil {
		fatal(err)
	}
	defer unix.Close(fd)
	mask := uint64(unix.FAN_OPEN | unix.FAN_MODIFY | unix.FAN_CLOSE_WRITE)
	if err := unix.FanotifyMark(fd, unix.FAN_MARK_ADD|unix.FAN_MARK_MOUNT, mask, unix.AT_FDCWD, root); err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	buf := make([]byte, 64*1024)
	for {
		n, err := unix.Read(fd, buf)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			fatal(err)
		}
		for offset := 0; offset+unix.FAN_EVENT_METADATA_LEN <= n; {
			meta := (*unix.FanotifyEventMetadata)(unsafe.Pointer(&buf[offset]))
			if meta.Event_len < unix.FAN_EVENT_METADATA_LEN {
				fatal(fmt.Errorf("invalid fanotify event length %d", meta.Event_len))
			}
			if meta.Fd >= 0 {
				path, _ := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", meta.Fd))
				if insideGit(path) {
					_ = enc.Encode(event{
						Time:    time.Now().UTC().Format(time.RFC3339Nano),
						PID:     meta.Pid,
						PPID:    processStatus(meta.Pid, "PPid:"),
						Mask:    maskName(meta.Mask),
						Path:    path,
						Command: commandLine(meta.Pid),
					})
				}
				_ = unix.Close(int(meta.Fd))
			}
			offset += int(meta.Event_len)
		}
	}
}

func insideGit(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.Contains(normalized, "/.git/") || strings.HasSuffix(normalized, "/.git")
}

func maskName(mask uint64) string {
	var names []string
	if mask&unix.FAN_OPEN != 0 {
		names = append(names, "OPEN")
	}
	if mask&unix.FAN_MODIFY != 0 {
		names = append(names, "MODIFY")
	}
	if mask&unix.FAN_CLOSE_WRITE != 0 {
		names = append(names, "CLOSE_WRITE")
	}
	return strings.Join(names, "|")
}

func commandLine(pid int32) string {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\x00", " "))
}

func processStatus(pid int32, key string) string {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
