//go:build !unix

package gui

// ClaimRunning is a no-op where flock is not available.
func ClaimRunning(string) (release func(), already bool, err error) {
	return func() {}, false, nil
}
