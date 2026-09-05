package clipboard

import "golang.org/x/sys/windows"

func testThreadID() uintptr { return uintptr(windows.GetCurrentThreadId()) }
