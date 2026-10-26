package api

import "testing"

func TestContextRequestOptionalParams(t *testing.T) {
	server := "http://example.com"

	t.Run("describe concept view", func(t *testing.T) {
		view := "short"
		req, err := NewCtxSchemeDescribeConceptRequest(server, "ctx-id", "v1",
			&CtxSchemeDescribeConceptParams{ConceptID: "C", CategoryID: "level", View: &view})
		if err != nil {
			t.Fatal(err)
		}
		query := req.URL.Query()
		if query.Get("conceptId") != "C" || query.Get("categoryId") != "level" || query.Get("view") != view {
			t.Fatalf("query = %q", req.URL.RawQuery)
		}
	})
}
