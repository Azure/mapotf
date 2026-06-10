package pkg

// IsLocalSourceForTest is a test-only re-export of the package-private
// isLocalSource helper so external _test packages can exercise it.
func IsLocalSourceForTest(source string) bool {
	return isLocalSource(source)
}
