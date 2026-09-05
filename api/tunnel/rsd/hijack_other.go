//go:build !darwin

package rsd

func hijack(f func() error) error {
	guard.Lock()
	defer guard.Unlock()
	return f()
}
