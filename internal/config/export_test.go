package config

// StubHostProbe replaces the DNS lookup and the route table path and returns
// a function that puts them back.
func StubHostProbe(lookup func(string) ([]string, error), routes string) func() {
	oldLookup, oldRoutes := lookupHost, routeFile
	lookupHost, routeFile = lookup, routes
	return func() { lookupHost, routeFile = oldLookup, oldRoutes }
}
