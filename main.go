package main

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"slices"
)

type allowMethodPathParams struct {
	Path        *regexp.Regexp
	Method      string
	AllowParams []string
}

func defaultValidator(r *http.Request) bool {
	rules := []allowMethodPathParams{
		{
			// CreateGraphDefs
			Path:        regexp.MustCompile("^/api/v0/graph-defs/create$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
		{
			// RetireHost
			Path:        regexp.MustCompile("^/api/v0/hosts/.*/retire$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
		{
			// FindHosts
			Path:        regexp.MustCompile("^/api/v0/hosts$"),
			Method:      http.MethodGet,
			AllowParams: []string{"customIdentifier", "status"},
		},
		{
			// PostCheckReports
			Path:        regexp.MustCompile("^/api/v0/monitoring/checks/report$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
		{
			// PutHostMetaData
			Path:        regexp.MustCompile("^/api/v0/hosts/.*/metadata/.*$"),
			Method:      http.MethodPut,
			AllowParams: nil,
		},
		{
			// UpdateHostStatus
			Path:        regexp.MustCompile("^/api/v0/hosts/.*/status$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
		{
			// UpdateHost
			Path:        regexp.MustCompile("^/api/v0/hosts/.*$"),
			Method:      http.MethodPut,
			AllowParams: nil,
		},
		{
			// PostHostMetricValues
			Path:        regexp.MustCompile("^/api/v0/tsdb$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
		{
			// FindHost
			Path:        regexp.MustCompile("^/api/v0/hosts/.*$"),
			Method:      http.MethodGet,
			AllowParams: nil,
		},
		{
			// CreateHost
			Path:        regexp.MustCompile("^/api/v0/hosts$"),
			Method:      http.MethodPost,
			AllowParams: nil,
		},
	}

	for idx := range rules {
		if rules[idx].Path.MatchString(r.URL.Path) && rules[idx].Method == r.Method {
			// nop
		} else {
			continue
		}

		if len(rules[idx].AllowParams) != len(r.URL.Query()) {
			continue
		}

		for k := range r.URL.Query() {
			if !slices.Contains(rules[idx].AllowParams, k) {
				return false
			}
		}
		return true
	}

	return false
}

type myHandler struct {
	dummyApiKey     string
	overWriteApiKey string
	validator       func(*http.Request) bool
}

func (h *myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rr := r.Clone(r.Context())
	if rr.Header.Get(apiKeyHeader) != h.dummyApiKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !h.validator(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	rr.RequestURI = ""
	rr.Host = "api.mackerelio.com"
	rr.URL.Host = "api.mackerelio.com"
	rr.URL.Scheme = "https"
	rr.Header.Set(apiKeyHeader, h.overWriteApiKey)

	resp, err := http.DefaultClient.Do(rr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for i, j := range resp.Header {
		for k := range j {
			w.Header().Add(i, j[k])
		}
	}

	b := &bytes.Buffer{}
	_, err = b.ReadFrom(resp.Body)
	if err != nil {
		log.Println(err)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(b.Bytes())
}

const (
	envOverWriteApiKey = "OVERWRITE_APIKEY"
	envDummyApiKey     = "DUMMY_APIKEY"
	apiKeyHeader       = "X-Api-Key"
)

func main() {
	apiKey := os.Getenv(envOverWriteApiKey)
	if apiKey == "" {
		fmt.Printf("need %s", envOverWriteApiKey)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr: ":8080",
		Handler: &myHandler{
			dummyApiKey:     cmp.Or(os.Getenv(envDummyApiKey), "DUMMY_APIKEY"),
			overWriteApiKey: apiKey,
			validator:       defaultValidator,
		},
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed
}
