package ezclassify_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Test-only shapes decode recordings, never the production wire contract.
type wireCase struct {
	ID        string `json:"id"`
	Operation string `json:"operationId"`
	Auth      struct {
		Credential string `json:"credential"`
	} `json:"auth"`
	Request struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
	} `json:"response"`
}
type wireCorpus struct {
	Cases   []wireCase `json:"cases"`
	Version string     `json:"corpusVersion"`
}
type wireManifest struct {
	Version string `json:"corpusVersion"`
	Spec    struct {
		Digest string `json:"digest"`
	} `json:"spec"`
	Coverage struct {
		Cases      int `json:"cases"`
		Operations int `json:"operationsCovered"`
	} `json:"coverage"`
	Divergences []json.RawMessage `json:"contractDivergences"`
}

func loadWireCorpus(specDir string) (wireCorpus, error) {
	var c wireCorpus
	var m wireManifest
	raw, e := os.ReadFile(filepath.Join(specDir, "wire/v1/corpus.json"))
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, e
	}
	raw, e = os.ReadFile(filepath.Join(specDir, "wire/v1/manifest.json"))
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &m); e != nil {
		return c, e
	}
	yaml, e := os.ReadFile(filepath.Join(specDir, "openapi.yaml"))
	if e != nil {
		return c, e
	}
	if c.Version != "v1" || m.Version != "v1" || m.Spec.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(yaml)) || len(m.Divergences) != 0 || m.Divergences == nil || m.Coverage.Cases != len(c.Cases) || len(c.Cases) == 0 {
		return c, fmt.Errorf("invalid wire provenance or coverage")
	}
	ids := map[string]bool{}
	ops := map[string]bool{}
	for _, r := range c.Cases {
		if r.ID == "" || ids[r.ID] {
			return c, fmt.Errorf("duplicate/empty wire id %q", r.ID)
		}
		ids[r.ID] = true
		if r.Operation != "" {
			ops[r.Operation] = true
		}
	}
	if len(ops) != m.Coverage.Operations {
		return c, fmt.Errorf("operation coverage mismatch")
	}
	return c, nil
}
func canonicalWireJSON(body []byte) (string, error) {
	if len(body) == 0 {
		return "(no body)", nil
	}
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if e := d.Decode(&v); e != nil {
		return "", e
	}
	var extra interface{}
	if e := d.Decode(&extra); e != io.EOF {
		return "", fmt.Errorf("trailing JSON")
	}
	b, e := json.Marshal(v)
	return string(b), e
}
func canonicalWireTarget(target string) string {
	path, query, _ := strings.Cut(target, "?")
	if path == "" {
		path = "/"
	} else if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if query == "" {
		return path
	}
	var pairs [][2]string
	for _, p := range strings.Split(query, "&") {
		if p == "" {
			continue
		}
		name, value, _ := strings.Cut(p, "=")
		pairs = append(pairs, [2]string{name, value})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i][0] == pairs[j][0] {
			return pairs[i][1] < pairs[j][1]
		}
		return pairs[i][0] < pairs[j][0]
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p[0]
		if p[1] != "" {
			parts[i] += "=" + p[1]
		}
	}
	return path + "?" + strings.Join(parts, "&")
}
func wireKey(method, target string, body []byte) (string, error) {
	v, e := canonicalWireJSON(body)
	return strings.ToUpper(method) + " " + canonicalWireTarget(target) + "\n" + v, e
}
func equalWireJSON(a, b []byte) bool {
	x, e := canonicalWireJSON(a)
	if e != nil {
		return false
	}
	y, e := canonicalWireJSON(b)
	return e == nil && x == y
}

// A replay serves only its single named exchange. Unexpected extra calls fail.
type wireReplay struct {
	mu              sync.Mutex
	Case            wireCase
	Hits            int
	Failure         error
	ObservedStatus  int
	ObservedBody    []byte
	ObservedHeaders http.Header
}

func (r *wireReplay) answer(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var body []byte
	var e error
	if req.Body != nil {
		body, e = io.ReadAll(req.Body)
	}
	if e != nil {
		return nil, e
	}
	got, e := wireKey(req.Method, req.URL.RequestURI(), body)
	if e != nil {
		return nil, e
	}
	want, e := wireKey(r.Case.Request.Method, r.Case.Request.Path, r.Case.Request.Body)
	if e != nil {
		return nil, e
	}
	if got != want {
		return nil, fmt.Errorf("%s changed request: %s != %s", r.Case.ID, got, want)
	}
	for k, v := range r.Case.Request.Headers {
		if req.Header.Get(k) != v {
			return nil, fmt.Errorf("%s changed %s", r.Case.ID, k)
		}
	}
	carrier, bearer := req.Header.Get("x-ezclassify-context-token"), req.Header.Get("Authorization")
	switch r.Case.Auth.Credential {
	case "context-token":
		if carrier != "fixture-context-carrier" || bearer != "" {
			return nil, fmt.Errorf("changed Context credential")
		}
	case "invalid-bearer":
		if carrier != "fixture-context-carrier" || bearer != "Bearer fixture-refused-bearer" {
			return nil, fmt.Errorf("changed refused credential")
		}
	case "none":
		if carrier != "" || bearer != "" {
			return nil, fmt.Errorf("leaked credential")
		}
	default:
		return nil, fmt.Errorf("unknown credential class")
	}
	if r.Hits != 0 {
		return nil, fmt.Errorf("duplicate request")
	}
	r.Hits++
	h := http.Header{}
	for k, v := range r.Case.Response.Headers {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: r.Case.Response.Status, Header: h, Body: io.NopCloser(bytes.NewReader(r.Case.Response.Body)), Request: req}, nil
}

type wireTransport struct {
	replay *wireReplay
	next   http.RoundTripper
}

func (w wireTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Accept", "application/json")
	var resp *http.Response
	var e error
	if w.next == nil {
		resp, e = w.replay.answer(req)
	} else {
		resp, e = w.next.RoundTrip(req)
	}
	if e != nil {
		return nil, e
	}
	body, e := io.ReadAll(resp.Body)
	resp.Body.Close()
	if e != nil {
		return nil, e
	}
	w.replay.ObservedStatus = resp.StatusCode
	w.replay.ObservedHeaders = resp.Header.Clone()
	w.replay.ObservedBody = append([]byte(nil), body...)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}
