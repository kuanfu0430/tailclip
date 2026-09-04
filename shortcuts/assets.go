// Package shortcutassets 提供 release 內嵌的兩支已簽署捷徑。
package shortcutassets

import _ "embed"

var (
	//go:embed dist/TailClip-Send.shortcut
	Send []byte

	//go:embed dist/TailClip-Pull.shortcut
	Pull []byte
)
