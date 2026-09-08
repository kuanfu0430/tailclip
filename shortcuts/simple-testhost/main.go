// simple-testhost 以合成記憶體剪貼簿驗證真正的臨時隧道，不讀寫桌面剪貼簿。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/simple"
)

func main() {
	state := flag.String("state", "build/simple-state.json", "私有測試狀態檔")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	memory := clipboard.NewMemory()
	service := simple.New("合成測試電腦", memory)
	defer service.Close()
	start, cancel := context.WithTimeout(ctx, 120*time.Second)
	err := service.Start(start)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	service.SetActive(true)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	writeState := func() error {
		pair, err := service.NewPairing()
		if err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{"url": "http://" + listener.Addr().String(), "pairing": pair})
		return os.WriteFile(*state, data, 0600)
	}
	if err := writeState(); err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var input struct{ Text string }
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&input) != nil {
				http.Error(w, "格式錯誤", 400)
				return
			}
			memory.WriteText(ctx, input.Text)
		}
		text, _ := memory.ReadText(ctx)
		json.NewEncoder(w).Encode(map[string]string{"text": text})
	})
	mux.HandleFunc("/pairing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if err := writeState(); err != nil {
			w.WriteHeader(500)
		}
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		service.Revoke()
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	go func() { <-ctx.Done(); server.Close() }()
	fmt.Println("真實臨時隧道已通過外部 HTTPS 健康檢查；僅使用合成測試資料")
	server.Serve(listener)
}
