//go:build !windows && !linux

package clipboard

import "context"

type unsupportedBackend struct{}

func NewSystemBackend() Backend { return unsupportedBackend{} }

func (unsupportedBackend) Available(context.Context) error { return ErrUnavailable }
func (unsupportedBackend) ReadText(context.Context) (string, error) {
	return "", ErrUnavailable
}
func (unsupportedBackend) WriteText(context.Context, string) error { return ErrUnavailable }
