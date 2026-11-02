package server

// SetBeforeSectionWriteForTest runs f between putSection's reads and its
// write's transaction (P-90) and returns the restore.
func SetBeforeSectionWriteForTest(f func()) (restore func()) {
	beforeSectionWrite = f
	return func() { beforeSectionWrite = func() {} }
}

// RouteTableForTest is every row of the route table as "METHOD pattern"
// (B-87), the signoff's turn routes included.
func RouteTableForTest() []string {
	s := &Server{}
	var out []string
	for _, rt := range append(s.routes(false, nil), append(s.apiRoutes(), s.signoffTurnRoutes()...)...) {
		out = append(out, rt.method+" "+rt.pattern)
	}
	return out
}
