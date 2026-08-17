package main

import (
	"embed"
	"net/http"
)

// The API documentation is compiled into the binary, so it deploys with the
// code it describes and cannot drift out of sync at runtime. Editing the spec
// requires a rebuild — which is correct, since the spec should change in the
// same commit as the endpoint it documents.
//
//go:embed docs/openapi.yaml docs/index.html
var docsFS embed.FS

// HandlerDocsPage serves Swagger UI. Public — the page must load before anyone
// can authenticate, and it contains no data.
func (cfg *apiConfig) HandlerDocsPage(w http.ResponseWriter, r *http.Request) {
	b, err := docsFS.ReadFile("docs/index.html")
	if err != nil {
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// HandlerOpenAPISpec serves the specification itself. Public for the same
// reason: Swagger UI fetches it before any token exists.
func (cfg *apiConfig) HandlerOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	b, err := docsFS.ReadFile("docs/openapi.yaml")
	if err != nil {
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
