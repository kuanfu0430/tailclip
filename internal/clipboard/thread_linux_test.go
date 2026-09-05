package clipboard

import "golang.org/x/sys/unix"

func testThreadID() uintptr { return uintptr(unix.Gettid()) }
