// Package shortcutassets 提供 release 內嵌的兩支已簽署捷徑。
package shortcutassets

import _ "embed"

var (
	//go:embed dist/TailBlink-Simple-Send.shortcut
	SimpleSend []byte

	//go:embed dist/TailBlink-Simple-Pull.shortcut
	SimplePull []byte

	//go:embed dist/TailBlink-Send.shortcut
	Send []byte

	//go:embed dist/TailBlink-Pull.shortcut
	Pull []byte
)
