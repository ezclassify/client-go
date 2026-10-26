// Package subsetmatch implements the recursive subset comparison shared by
// the conformance runner and the replayer-contract test.  It mirrors the Node
// reference runner's subsetMatch logic, so all SDKs assert expected values the
// same way.
package subsetmatch

import "reflect"

// Match reports whether every key in expected is equal in actual.  Objects
// are compared key-by-key (actual may carry extra keys); arrays are matched
// element-by-element and actual may have more elements.  An expected key
// with a null value also matches a key absent from actual, because a missing
// key reads as nil; null does not pin key existence.  This mirrors the Node
// reference matcher, so all SDKs assert the same way.  Both values are
// expected to be the output of json.Unmarshal into interface{} (nil, bool,
// float64, string, []interface{}, map[string]interface{}).
func Match(actual, expected interface{}) bool {
	if expected == nil {
		return actual == nil
	}
	switch e := expected.(type) {
	case bool:
		a, ok := actual.(bool)
		return ok && a == e
	case float64:
		switch a := actual.(type) {
		case float64:
			return a == e
		case int:
			return float64(a) == e
		}
		return false
	case string:
		a, ok := actual.(string)
		return ok && a == e
	case []interface{}:
		a, ok := actual.([]interface{})
		if !ok || len(a) < len(e) {
			return false
		}
		for i, ev := range e {
			if !Match(a[i], ev) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for k, ev := range e {
			if !Match(a[k], ev) {
				return false
			}
		}
		return true
	}
	// Unreachable for json.Unmarshal output; kept for hand-built expected
	// values (e.g. untyped ints in Go test literals) so those still compare.
	return reflect.DeepEqual(actual, expected)
}
