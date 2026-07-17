//go:build unix

package config

import (
	"fmt"
	"os"
	"syscall"
)

func checkOwner(st os.FileInfo) error {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(sys.Uid) != os.Getuid() {
		return fmt.Errorf("not owned by the current user (uid %d, expected %d)", sys.Uid, os.Getuid())
	}
	return nil
}
