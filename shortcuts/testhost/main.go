// testhost 只供 macOS 原生 Shortcuts 整合測試，使用獨立記憶體剪貼簿。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kuanfu0430/tailblink/internal/api"
	"github.com/kuanfu0430/tailblink/internal/clipboard"
	"github.com/kuanfu0430/tailblink/internal/config"
)

type source struct{ config.Config }

func (s source) Snapshot() config.Config { return s.Config }

func main() {
	state := flag.String("state", "", "測試狀態輸出檔案（必填）")
	flag.Parse()
	if *state == "" {
		panic("需要 --state")
	}
	// 由 OS 原子選擇未佔用的 loopback port，絕不監聽所有介面。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	memory := clipboard.NewMemory()
	token := strings.Repeat("A", 43) // 公開、無權限的合成測試值。
	handler := api.New(api.Options{Config: source{config.Config{
		Version: 1, DeviceName: "隔離測試電腦", TailscaleDevice: "fixture", PairingToken: token,
	}}, Clipboard: memory, RateLimit: 10000}).Handler()
	var mu sync.Mutex
	requests := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost {
			var input struct{ Text string }
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&input) != nil {
				http.Error(w, "invalid fixture", 400)
				return
			}
			_ = memory.WriteText(context.Background(), input.Text)
			requests = []string{}
		}
		text, _ := memory.ReadText(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"text": text, "requests": requests})
	})
	mux.Handle("/tailblink/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		http.StripPrefix("/tailblink", handler).ServeHTTP(w, r)
	}))
	url := "http://" + listener.Addr().String()
	data, _ := json.Marshal(map[string]any{"url": url, "pairing": map[string]any{
		"version": 1, "base_url": url + "/tailblink/v1", "token": token,
		"device_name": "隔離測試電腦", "tailscale_device": "fixture",
	}})
	if err := os.WriteFile(*state, data, 0600); err != nil {
		panic(err)
	}
	fmt.Println("原生捷徑測試服務已啟動（僅 127.0.0.1；記憶體測試資料）")
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}
