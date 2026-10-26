package ezclassify

import (
	"encoding/json"
	"reflect"
	"time"
)

// SetCacheClockForTest replaces the clock the Context's response cache reads
// for entry lifetimes. It is compiled only into tests and moves nothing else:
// request timeouts keep running on real time.
func SetCacheClockForTest(cx *EzClassifyContext, now func() time.Time) {
	cx.cache.mu.Lock()
	cx.cache.now = now
	cx.cache.mu.Unlock()
}

// CacheWaitersForTest reports how many callers are blocked on in-flight cache
// entries of the Context, so a test can tell that every caller has joined.
func CacheWaitersForTest(cx *EzClassifyContext) int {
	cache := cx.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	total := 0
	for _, entry := range cache.entries {
		if entry.pending {
			total += entry.waiters
		}
	}
	return total
}

// ZeroFillRequiredForTest returns raw with every member the generated type
// requires but raw leaves out set to its zero value, at every depth. Tests that
// stub a response to exercise routing, headers or one field use it so the
// stub still satisfies the response contract the client now enforces, without
// restating members the test does not examine.
func ZeroFillRequiredForTest(valueType reflect.Type, raw []byte) []byte {
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return raw
	}
	filled, err := json.Marshal(fillRequired(valueType, tree))
	if err != nil {
		return raw
	}
	return filled
}

func fillRequired(valueType reflect.Type, value any) any {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		if unionVariantTypes(valueType) != nil {
			return value
		}
		object, ok := value.(map[string]any)
		if !ok {
			return value
		}
		fillObject(valueType, object)
	case reflect.Slice, reflect.Array:
		if items, ok := value.([]any); ok {
			for i := range items {
				items[i] = fillRequired(valueType.Elem(), items[i])
			}
		}
	case reflect.Map:
		if entries, ok := value.(map[string]any); ok {
			for key := range entries {
				entries[key] = fillRequired(valueType.Elem(), entries[key])
			}
		}
	}
	return value
}

func fillObject(structType reflect.Type, object map[string]any) {
	for _, plan := range planFields(structType) {
		if plan.inline {
			inner := plan.typ
			for inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			if inner.Kind() == reflect.Struct {
				fillObject(inner, object)
			}
			continue
		}
		member, present := object[plan.name]
		if plan.required && (!present || (member == nil && !nullable(plan.typ))) {
			object[plan.name] = zeroMember(plan.typ)
			continue
		}
		if member != nil {
			object[plan.name] = fillRequired(plan.typ, member)
		}
	}
}

func zeroMember(valueType reflect.Type) any {
	switch valueType.Kind() {
	case reflect.Pointer, reflect.Interface:
		return nil
	case reflect.String:
		return ""
	case reflect.Bool:
		return false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return 0
	case reflect.Slice, reflect.Array:
		return []any{}
	case reflect.Map:
		return map[string]any{}
	case reflect.Struct:
		return fillRequired(valueType, map[string]any{})
	}
	return nil
}
