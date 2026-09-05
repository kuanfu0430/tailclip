package clipboard

import "golang.org/x/sys/unix"

func testThreadID() uintptr {
	id, _, _ := unix.RawSyscall(unix.SYS_THREAD_SELFID, 0, 0, 0)
	return id
}
