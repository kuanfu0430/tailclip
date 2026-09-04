package shortcutassets

import "testing"

func TestSignedShortcutsAreEmbedded(t *testing.T) {
	if len(Send) < 1024 {
		t.Fatalf("傳送捷徑過小或未嵌入：%d bytes", len(Send))
	}
	if len(Pull) < 1024 {
		t.Fatalf("取回捷徑過小或未嵌入：%d bytes", len(Pull))
	}
}
