package tailscale

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs [][]byte
	errors  []error
	calls   [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	index := len(f.calls) - 1
	var output []byte
	if index < len(f.outputs) {
		output = f.outputs[index]
	}
	if index < len(f.errors) {
		return output, f.errors[index]
	}
	return output, nil
}

func TestStatusAndBaseURL(t *testing.T) {
	runner := &fakeRunner{outputs: [][]byte{[]byte(`{"BackendState":"Running","Self":{"DNSName":"work.example.ts.net.","HostName":"work"}}`)}}
	status, err := NewWithRunner(runner).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := BaseURL(status); got != "https://work.example.ts.net/tailclip/v1" {
		t.Fatalf("base URL = %s", got)
	}
}

func TestEnsureServePreservesExistingRoot(t *testing.T) {
	status := []byte(`{"Web":{"work.example.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:8080"}}}}}`)
	runner := &fakeRunner{outputs: [][]byte{status, nil}}
	changed, err := NewWithRunner(runner).EnsureServe(context.Background())
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	want := []string{"serve", "--bg", "--yes", "--https=443", "--set-path=/tailclip", AgentTarget}
	if len(runner.calls) != 2 || !reflect.DeepEqual(runner.calls[1], want) {
		t.Fatalf("calls=%v", runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(strings.Join(call, " "), "reset") {
			t.Fatal("不得呼叫 serve reset")
		}
	}
}

func TestEnsureServeIsIdempotentAndRejectsConflict(t *testing.T) {
	ours := []byte(`{"Web":{"work.example.ts.net:443":{"Handlers":{"/tailclip":{"Proxy":"http://127.0.0.1:17733/"}}}}}`)
	runner := &fakeRunner{outputs: [][]byte{ours}}
	changed, err := NewWithRunner(runner).EnsureServe(context.Background())
	if err != nil || changed || len(runner.calls) != 1 {
		t.Fatalf("changed=%v err=%v calls=%v", changed, err, runner.calls)
	}

	other := []byte(`{"Web":{"work.example.ts.net:443":{"Handlers":{"/tailclip":{"Proxy":"http://127.0.0.1:9999"}}}}}`)
	runner = &fakeRunner{outputs: [][]byte{other}}
	changed, err = NewWithRunner(runner).EnsureServe(context.Background())
	if changed || !errors.Is(err, ErrServeConflict) || len(runner.calls) != 1 {
		t.Fatalf("changed=%v err=%v calls=%v", changed, err, runner.calls)
	}
}

func TestRemoveServeOnlyRemovesOurMapping(t *testing.T) {
	ours := []byte(`{"Web":{"work.example.ts.net:443":{"Handlers":{"/tailclip":{"Proxy":"http://127.0.0.1:17733"}}}}}`)
	runner := &fakeRunner{outputs: [][]byte{ours, nil}}
	changed, err := NewWithRunner(runner).RemoveServe(context.Background())
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	want := []string{"serve", "--yes", "--https=443", "--set-path=/tailclip", "off"}
	if !reflect.DeepEqual(runner.calls[1], want) {
		t.Fatalf("call=%v", runner.calls[1])
	}
}
