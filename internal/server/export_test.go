package server

// SetBeforeSectionWriteForTest runs f between putSection's reads and its
// write's transaction (P-90) and returns the restore.
func SetBeforeSectionWriteForTest(f func()) (restore func()) {
	beforeSectionWrite = f
	return func() { beforeSectionWrite = func() {} }
}
