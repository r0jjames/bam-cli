package bamboo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// route answers one "METHOD /path" with a fixture file or an inline body.
type route struct {
	status  int    // default 200
	fixture string // file under testdata/
	body    string // used when fixture is empty
	header  map[string]string
	times   int // answer with this route only for the first N calls (0 = always)
	then    *route
}

type recorded struct {
	mu   sync.Mutex
	reqs []*http.Request
	form []string // encoded bodies, in order
}

func (r *recorded) all() []*http.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*http.Request(nil), r.reqs...)
}

// newTestServer serves routes and returns a Client pointed at it. Sleeps are
// instant so retry tests do not wait.
func newTestServer(t *testing.T, routes map[string]*route) (*Client, *recorded) {
	t.Helper()
	rec := &recorded{}
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = req.ParseForm()
		// A route may name the expand parameter: "GET /path?expand=variables".
		key := req.Method + " " + req.URL.Path
		if exp := req.URL.Query().Get("expand"); exp != "" {
			if _, ok := routes[key+"?expand="+exp]; ok {
				key += "?expand=" + exp
			}
		}
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, req)
		rec.form = append(rec.form, req.PostForm.Encode())
		calls[key]++
		n := calls[key]
		rec.mu.Unlock()

		rt, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"no route ` + key + `"}`))
			return
		}
		for rt.times > 0 && n > rt.times && rt.then != nil {
			n -= rt.times
			rt = rt.then
		}
		for k, v := range rt.header {
			w.Header().Set(k, v)
		}
		status := rt.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		body := []byte(rt.body)
		if rt.fixture != "" {
			var err error
			body, err = os.ReadFile(filepath.Join("testdata", rt.fixture))
			if err != nil {
				t.Errorf("fixture %s: %v", rt.fixture, err)
			}
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c, err := New(Options{
		BaseURL: srv.URL,
		Token:   "test-token",
		Now:     func() time.Time { return time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC) },
		Sleep:   func(context.Context, time.Duration) error { return nil },
	})
	require.NoError(t, err)
	return c, rec
}
