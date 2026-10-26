// Package replayer is a stub HTTP server that serves pre-recorded engine
// responses during conformance tests.  It implements the replayer-contract
// defined in spec/replayer-contract/v1/README.md.
//
// Lookup algorithm: sha256(normalizedPath + "|" + stableJSON(body)).
// A miss returns HTTP 404 with the envelope: {error, message, path, bodyHash}.
package replayer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
)

// Recording holds one pre-recorded response entry.
type Recording struct {
	// StatusCode is the HTTP status to return (typically 200).
	StatusCode int
	// Body is the raw JSON response body.
	Body json.RawMessage
}

// Replayer is a stub HTTP server backed by a recording map.
// Keys in the recording map are sha256(normalizedPath+"|"+stableJSON(body)).
type Replayer struct {
	recordings map[string]Recording
	server     *httptest.Server
}

// New creates and immediately starts a Replayer loaded with the given recordings.
// Call Close when done to shut down the underlying HTTP server.
func New(recordings map[string]Recording) *Replayer {
	r := &Replayer{recordings: recordings}
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handle)
	r.server = httptest.NewServer(mux)
	return r
}

// URL returns the base URL of the running HTTP server (e.g. "http://127.0.0.1:NNNNN").
func (r *Replayer) URL() string {
	return r.server.URL
}

// Close shuts down the HTTP server.
func (r *Replayer) Close() {
	r.server.Close()
}

func (r *Replayer) handle(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimRight(req.URL.Path, "/")

	var bodyVal interface{}
	if err := json.NewDecoder(req.Body).Decode(&bodyVal); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"error":"invalid_json","message":%s}`, jsonString(err.Error()))
		return
	}

	stable := StableJSON(bodyVal)
	hashInput := path + "|" + stable
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(hashInput)))

	rec, ok := r.recordings[hash]
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		bodyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(stable)))
		w.WriteHeader(http.StatusNotFound)
		// The path goes through jsonString in the message too: req.URL.Path is
		// percent-decoded, so a quote or backslash would otherwise break the
		// envelope's JSON.
		_, _ = fmt.Fprintf(w,
			`{"error":"no_recording","message":%s,"path":%s,"bodyHash":%s}`,
			jsonString("no recording for path="+path), jsonString(path), jsonString(bodyHash),
		)
		return
	}

	w.WriteHeader(rec.StatusCode)
	_, _ = w.Write(rec.Body)
}

// StableJSON serializes v as compact JSON with all object keys sorted
// lexicographically (byte order of their UTF-8 encoding).  Arrays are not
// reordered.  This is the canonical form used for the replayer lookup key.
func StableJSON(v interface{}) string {
	switch x := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			kJSON, _ := json.Marshal(k)
			parts[i] = string(kJSON) + ":" + StableJSON(x[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []interface{}:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = StableJSON(item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// RecordingKey computes the replayer lookup key for a given path and body map.
// It is exported so the conformance test can pre-populate the recording map
// with entries that match the requests the SDK will send.
func RecordingKey(path string, body interface{}) string {
	normalizedPath := strings.TrimRight(path, "/")
	stable := StableJSON(body)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(normalizedPath+"|"+stable)))
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
