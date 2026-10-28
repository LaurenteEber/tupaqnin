//go:build !devroutes

package server

import "net/http"

// Without -tags=devroutes there are no development routes (B-4).
func registerDevRoutes(_ *http.ServeMux, _ *Server) {}
