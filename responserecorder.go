package main

import "net/http"

// responseRecorder wraps an http.ResponseWriter to capture the status code.
// The interface is write-only — WriteHeader serialises the status onto the
// connection and net/http keeps no record of it — so middleware that wants to
// log the status has to intercept the call on the way past.
//
// The embedded interface means Header and Write are promoted and forwarded
// untouched; only WriteHeader is overridden, and it still calls through to the
// real writer. Behaviour on the wire is unchanged.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *responseRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}
