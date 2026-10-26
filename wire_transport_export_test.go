package ezclassify

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// WireReplayRequestForTest exposes the existing credentialed transport only to
// external repository tests for malformed bodies and routes without an SDK
// operation. It is absent from the shipped package and adds no public raw API.
func WireReplayRequestForTest(ctx context.Context, c *Client, method, target string, headers map[string]string, body []byte) ([]byte, int, error) {
	req, e := http.NewRequestWithContext(ctx, method, c.baseURL+target, bytes.NewReader(body))
	if e != nil {
		return nil, 0, e
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, e := c.httpClient.Do(req)
	if e != nil {
		return nil, 0, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	return b, resp.StatusCode, e
}
