package clipboard

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrUnavailable = errors.New("剪貼簿不可用")
	ErrBusy        = errors.New("剪貼簿忙碌")
	ErrNoText      = errors.New("剪貼簿沒有文字")
)

// Backend 是 M1 唯一的平台抽象，刻意只支援純文字。
type Backend interface {
	Available(context.Context) error
	ReadText(context.Context) (string, error)
	WriteText(context.Context, string) error
}

// Synchronized 確保狀態查詢與讀寫不會在同一程序內互相搶用系統剪貼簿。
type Synchronized struct {
	backend Backend
	mu      sync.Mutex
}

func NewSynchronized(backend Backend) *Synchronized {
	return &Synchronized{backend: backend}
}

func (s *Synchronized) Available(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.backend.Available(ctx)
}

func (s *Synchronized) ReadText(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.backend.ReadText(ctx)
}

func (s *Synchronized) WriteText(ctx context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.backend.WriteText(ctx, text)
}

// Memory 是自動測試使用的記憶體剪貼簿。
type Memory struct {
	mu        sync.RWMutex
	text      string
	available bool
}

func NewMemory() *Memory {
	return &Memory{available: true}
}

func (m *Memory) Available(context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.available {
		return ErrUnavailable
	}
	return nil
}

func (m *Memory) ReadText(context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.available {
		return "", ErrUnavailable
	}
	if m.text == "" {
		return "", ErrNoText
	}
	return m.text, nil
}

func (m *Memory) WriteText(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.available {
		return ErrUnavailable
	}
	m.text = text
	return nil
}

func (m *Memory) SetAvailable(available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = available
}
