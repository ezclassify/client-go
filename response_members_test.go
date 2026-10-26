package ezclassify

import (
	"encoding/json"
	"reflect"
	"testing"
)

type memberLeaf struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

type memberVariantA struct {
	Kind string `json:"kind"`
	A    string `json:"a"`
}

type memberVariantB struct {
	Kind string `json:"kind"`
	B    string `json:"b"`
}

// memberUnion has the shape oapi-codegen gives a oneOf: raw state behind an
// AsX method per alternative.
type memberUnion struct{ union json.RawMessage }

func (u memberUnion) AsMemberVariantA() (memberVariantA, error) { return memberVariantA{}, nil }
func (u memberUnion) AsMemberVariantB() (memberVariantB, error) { return memberVariantB{}, nil }

type memberRoot struct {
	Leaf     memberLeaf            `json:"leaf"`
	Leaves   []memberLeaf          `json:"leaves"`
	ByKey    map[string]memberLeaf `json:"byKey"`
	Nullable *memberLeaf           `json:"nullable"`
	Optional *memberLeaf           `json:"optional,omitempty"`
	Choice   memberUnion           `json:"choice"`
	memberLeaf
}

const memberRootComplete = `{"leaf":{"name":"a"},"leaves":[{"name":"b"}],"byKey":{"k":{"name":"c"}},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"inline"}`

func TestMissingRequiredMember(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"complete body", memberRootComplete, ""},
		{"required member absent", `{"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/leaf"},
		{"nested member absent", `{"leaf":{},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/leaf/name"},
		{"array element member absent", `{"leaf":{"name":"a"},"leaves":[{"name":"b"},{}],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/leaves/1/name"},
		{"map value member absent", `{"leaf":{"name":"a"},"leaves":[],"byKey":{"a/b":{}},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/byKey/a~1b/name"},
		{"null where the type is not nullable", `{"leaf":null,"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/leaf"},
		{"null array is a missing array", `{"leaf":{"name":"a"},"leaves":null,"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n"}`, "/leaves"},
		{"nullable member present as null", `{"leaf":{"name":"a"},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"b","b":"x"},"name":"n"}`, ""},
		{"nullable member absent", `{"leaf":{"name":"a"},"leaves":[],"byKey":{},"choice":{"kind":"a","a":"x"},"name":"n"}`, "/nullable"},
		{"union alternative satisfied", `{"leaf":{"name":"a"},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"b","b":"x"},"name":"n"}`, ""},
		{"no union alternative satisfied", `{"leaf":{"name":"a"},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"c"},"name":"n"}`, "/choice/a"},
		{"inline member absent", `{"leaf":{"name":"a"},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"}}`, "/name"},
		{"unknown members are tolerated", `{"leaf":{"name":"a","future":1},"leaves":[],"byKey":{},"nullable":null,"choice":{"kind":"a","a":"x"},"name":"n","extra":true}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := missingRequiredMember(reflect.TypeOf(memberRoot{}), []byte(test.body)); got != test.want {
				t.Errorf("missing member = %q, want %q", got, test.want)
			}
		})
	}
}
