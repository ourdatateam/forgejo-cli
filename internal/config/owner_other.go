//go:build !unix

package config

import "os"

func checkOwner(st os.FileInfo) error { return nil }
