package ezclassify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// MalformedResponseError is returned when the rest-api answers 2xx with a body
// that does not satisfy the response contract: a member the contract requires
// is absent or null at some depth, or the body could not be decoded at all. The
// generated decoder would fill an absent member with its zero value and report
// success, so the client checks the body itself and refuses to hand the caller
// (or the response cache) an answer that was never given.
//
// It is distinct from [APIError], which covers non-2xx statuses only.
type MalformedResponseError struct {
	// Op names the SDK operation whose response was malformed.
	Op string
	// StatusCode is the HTTP status of the malformed response.
	StatusCode int
	// Member is the JSON pointer of the first required member the body lacks,
	// such as "/result/valid". It is empty when the body could not be decoded.
	Member string
}

// Error describes the malformation without echoing the response body.
func (e *MalformedResponseError) Error() string {
	prefix := "ezclassify: " + e.Op + ": rest-api returned " + strconv.Itoa(e.StatusCode)
	if e.Member == "" {
		return prefix + " with a response the client could not decode"
	}
	return prefix + " with a response missing required member " + e.Member
}

// successResult returns val, the decoded body of a response a client-level
// operation received, when the response is a 2xx that satisfies the contract.
// A non-2xx is an [APIError]; a 2xx the generated parser produced no value for,
// or whose body lacks a required member, is a [MalformedResponseError].
func successResult[T any](op string, resp *http.Response, status int, body []byte, val *T) (*T, error) {
	if status < 200 || status >= 300 {
		return nil, newAPIError(op, status, headerOf(resp), body)
	}
	if val == nil {
		return nil, &MalformedResponseError{Op: op, StatusCode: status}
	}
	if err := requireMembers[T](op, status, body); err != nil {
		return nil, err
	}
	return val, nil
}

// responseStatus records the status of the last response the client's
// transport received for one call. The generated parsers drop the response
// when they fail to decode its body, so this is how a decode failure learns
// whether the body it refused belonged to a success.
type responseStatus struct{ code atomic.Int32 }

type responseStatusKey struct{}

// withResponseStatus returns ctx carrying a fresh recorder for the call made
// with it.
func withResponseStatus(ctx context.Context) (context.Context, *responseStatus) {
	status := &responseStatus{}
	return context.WithValue(ctx, responseStatusKey{}, status), status
}

// recordResponseStatus stores code in the recorder ctx carries, if any.
func recordResponseStatus(ctx context.Context, code int) {
	if status, ok := ctx.Value(responseStatusKey{}).(*responseStatus); ok {
		status.code.Store(int32(code))
	}
}

// decodeFailure turns the generated decoder's refusal of a 2xx body into a
// [MalformedResponseError]. Any other error, including a decode failure on an
// error status and a body cut off by the response ceiling, is returned as-is.
func decodeFailure(op string, status *responseStatus, err error) error {
	code := int(status.code.Load())
	if code < 200 || code >= 300 {
		return err
	}
	var syntax *json.SyntaxError
	var mistyped *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &mistyped) {
		return &MalformedResponseError{Op: op, StatusCode: code}
	}
	return err
}

// requireMembers refuses a 2xx body that lacks a member the contract requires
// of T. The generated decoder fills such a member with its zero value, which
// reads as an answer the server never gave, and a cached one would outlive the
// call that produced it.
func requireMembers[T any](op string, status int, body []byte) error {
	if member := missingRequiredMember(reflect.TypeOf((*T)(nil)).Elem(), body); member != "" {
		return &MalformedResponseError{Op: op, StatusCode: status, Member: member}
	}
	return nil
}

// missingRequiredMember returns the JSON pointer of the first member of body
// that the contract requires and body lacks, or "" when the body satisfies it.
// valueType is the generated type the body decodes into.
//
// The generated types carry the contract: oapi-codegen leaves omitempty off a
// member exactly when the schema requires it, and a required member that may be
// null is a pointer. Reading requirements from the types keeps them in step
// with the vendored OpenAPI document without a second copy of the schema, and
// no domain value appears in the check.
func missingRequiredMember(valueType reflect.Type, body []byte) string {
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		return ""
	}
	return missingMember(valueType, tree, "")
}

var (
	fieldPlans    sync.Map // reflect.Type -> []fieldPlan
	unionVariants sync.Map // reflect.Type -> []reflect.Type
)

// fieldPlan is how one exported struct field appears on the wire.
type fieldPlan struct {
	name     string
	typ      reflect.Type
	required bool
	inline   bool
}

func planFields(structType reflect.Type) []fieldPlan {
	if cached, ok := fieldPlans.Load(structType); ok {
		return cached.([]fieldPlan)
	}
	var plans []fieldPlan
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag, tagged := field.Tag.Lookup("json")
		if tag == "-" || (!field.IsExported() && !field.Anonymous) {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		if field.Anonymous && !tagged {
			plans = append(plans, fieldPlan{typ: field.Type, inline: true})
			continue
		}
		if !tagged || name == "" {
			continue
		}
		plans = append(plans, fieldPlan{
			name:     name,
			typ:      field.Type,
			required: !strings.Contains(","+options+",", ",omitempty,"),
		})
	}
	fieldPlans.Store(structType, plans)
	return plans
}

// unionVariantTypes lists the alternatives of an oapi-codegen oneOf wrapper: a
// struct whose only state is the unexported raw union, with an AsX method per
// alternative. It returns nil for any other type.
func unionVariantTypes(structType reflect.Type) []reflect.Type {
	if cached, ok := unionVariants.Load(structType); ok {
		return cached.([]reflect.Type)
	}
	var variants []reflect.Type
	if structType.NumField() == 1 && structType.Field(0).Name == "union" {
		for i := 0; i < structType.NumMethod(); i++ {
			method := structType.Method(i)
			if !strings.HasPrefix(method.Name, "As") || method.Type.NumIn() != 1 || method.Type.NumOut() != 2 {
				continue
			}
			variants = append(variants, method.Type.Out(0))
		}
	}
	unionVariants.Store(structType, variants)
	return variants
}

func missingMember(valueType reflect.Type, value any, path string) string {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		if variants := unionVariantTypes(valueType); variants != nil {
			return missingFromUnion(variants, value, path)
		}
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		return missingFromObject(valueType, object, path)
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return ""
		}
		for i, item := range items {
			if missing := missingMember(valueType.Elem(), item, path+"/"+strconv.Itoa(i)); missing != "" {
				return missing
			}
		}
	case reflect.Map:
		entries, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		for key, entry := range entries {
			if missing := missingMember(valueType.Elem(), entry, path+"/"+escapePointer(key)); missing != "" {
				return missing
			}
		}
	}
	return ""
}

func missingFromObject(structType reflect.Type, object map[string]any, path string) string {
	for _, plan := range planFields(structType) {
		if plan.inline {
			inner := plan.typ
			for inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			if inner.Kind() == reflect.Struct {
				if missing := missingFromObject(inner, object, path); missing != "" {
					return missing
				}
			}
			continue
		}
		member, present := object[plan.name]
		memberPath := path + "/" + escapePointer(plan.name)
		if member == nil && plan.required && !(present && nullable(plan.typ)) {
			return memberPath
		}
		if member == nil {
			continue
		}
		if missing := missingMember(plan.typ, member, memberPath); missing != "" {
			return missing
		}
	}
	return ""
}

// missingFromUnion accepts a value that satisfies any one alternative. A union
// with an alternative that is not an object cannot be judged by member
// presence, so it is accepted. A value no alternative satisfies is reported
// against the alternative it fits best, the one that declares the most of the
// members the value carries, so the error names what that answer lacks rather
// than what a different kind of answer would have needed.
func missingFromUnion(variants []reflect.Type, value any, path string) string {
	object, isObject := value.(map[string]any)
	if !isObject {
		return ""
	}
	best, bestFit := "", -1
	for _, variant := range variants {
		resolved := variant
		for resolved.Kind() == reflect.Pointer {
			resolved = resolved.Elem()
		}
		if resolved.Kind() != reflect.Struct {
			return ""
		}
		missing := missingMember(variant, value, path)
		if missing == "" {
			return ""
		}
		if fit := declaredMembersPresent(resolved, object); fit > bestFit {
			best, bestFit = missing, fit
		}
	}
	return best
}

// declaredMembersPresent counts the members of object that structType declares.
func declaredMembersPresent(structType reflect.Type, object map[string]any) int {
	count := 0
	for _, plan := range planFields(structType) {
		if plan.inline {
			inner := plan.typ
			for inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			if inner.Kind() == reflect.Struct {
				count += declaredMembersPresent(inner, object)
			}
			continue
		}
		if _, present := object[plan.name]; present {
			count++
		}
	}
	return count
}

// nullable reports whether the generated type can hold a JSON null as a value
// of its own: a required member typed this way is one the schema allows to be
// null, as opposed to a value type, where null means the member was not given.
func nullable(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer || t.Kind() == reflect.Interface
}

func escapePointer(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
