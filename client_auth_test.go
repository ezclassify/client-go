package ezclassify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRestClientCredentialsAreScopedByRoute(t *testing.T) {
	type observedHeaders struct {
		authorization string
		contextToken  string
	}
	seen := make(map[string]observedHeaders)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = observedHeaders{
			authorization: r.Header.Get("Authorization"),
			contextToken:  r.Header.Get(contextTokenHeader),
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/health":
			_, _ = io.WriteString(w, `{"status":"ok","engineVersion":"1.0.0","engineSha":"sha","gitTag":"tag","openapiVersion":"1.0.0","abiVersion":"v1"}`)
		case "/v1/ctx":
			_, _ = io.WriteString(w, `{"contexts":[]}`)
		case "/v1/ctx/example":
			_, _ = io.WriteString(w, `{"id":"example","versions":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := New(srv.URL, WithCredentials(RestClientCredentials{
		AccessToken:  "caller-access",
		ContextToken: "signed-context",
	}))
	ctx := context.Background()
	if _, err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListContexts(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListContextVersions(ctx, "example"); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/v1/health", "/v1/ctx", "/v1/ctx/example"} {
		if got := seen[path].authorization; got != "Bearer caller-access" {
			t.Errorf("%s Authorization = %q", path, got)
		}
	}
	if got := seen["/v1/health"].contextToken; got != "" {
		t.Errorf("health Context token = %q, want absent", got)
	}
	for _, path := range []string{"/v1/ctx", "/v1/ctx/example"} {
		if got := seen[path].contextToken; got != "signed-context" {
			t.Errorf("%s Context token = %q", path, got)
		}
	}
}

func TestRestClientCredentialsRejectBlankValues(t *testing.T) {
	for _, credentials := range []RestClientCredentials{
		{AccessToken: "  \t"},
		{ContextToken: "\n"},
	} {
		t.Run(strings.ReplaceAll(credentials.AccessToken+credentials.ContextToken, "\n", "newline"), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("WithCredentials did not reject a blank credential")
				}
			}()
			_ = WithCredentials(credentials)
		})
	}
}

func TestContextTokenSurvivesABaseURLPathPrefix(t *testing.T) {
	var contextTokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			t.Errorf("request path %q lost the base-URL prefix", r.URL.Path)
		}
		contextTokens = append(contextTokens, r.Header.Get(contextTokenHeader))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"contexts":[]}`)
	}))
	t.Cleanup(srv.Close)

	client := New(srv.URL+"/api", WithCredentials(RestClientCredentials{ContextToken: "signed-context"}))
	if _, err := client.ListContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(contextTokens) != 1 {
		t.Fatalf("server saw %d requests, want 1", len(contextTokens))
	}
	if contextTokens[0] != "signed-context" {
		t.Errorf("Context token = %q, want it attached under the path prefix", contextTokens[0])
	}
}

func TestNewRejectsAMalformedBaseURL(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("constructor accepted a malformed base URL")
		}
	}()
	_ = New("not a url", WithHTTPClient(&http.Client{}), WithCredentials(RestClientCredentials{}))
}

func TestCredentialsDoNotFollowACrossOriginRedirect(t *testing.T) {
	type observed struct{ authorization, contextToken string }
	var elsewhere observed
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere = observed{r.Header.Get("Authorization"), r.Header.Get(contextTokenHeader)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"contexts":[]}`)
	}))
	t.Cleanup(target.Close)

	var atOrigin observed
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atOrigin = observed{r.Header.Get("Authorization"), r.Header.Get(contextTokenHeader)}
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	client := New(origin.URL, WithCredentials(RestClientCredentials{
		AccessToken:  "caller-access",
		ContextToken: "signed-context",
	}))
	if _, err := client.ListContexts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atOrigin.authorization != "Bearer caller-access" || atOrigin.contextToken != "signed-context" {
		t.Fatalf("configured origin saw %+v, want both credentials", atOrigin)
	}
	if elsewhere.authorization != "" || elsewhere.contextToken != "" {
		t.Fatalf("redirect target saw %+v, want neither credential", elsewhere)
	}
}

func TestRestClientCredentialsRejectControlCharacters(t *testing.T) {
	for name, credentials := range map[string]RestClientCredentials{
		"access token with a trailing newline":  {AccessToken: "token\n"},
		"context token with a trailing newline": {ContextToken: "token\n"},
		"access token with an embedded null":    {AccessToken: "tok\x00en"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("WithCredentials accepted a token that cannot be sent as a header")
				}
			}()
			_ = WithCredentials(credentials)
		})
	}
}
