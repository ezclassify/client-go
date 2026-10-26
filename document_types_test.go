package ezclassify

import (
	"reflect"
	"testing"
)

// compileDocumentSpanOffsetTypes is never called; compiling it locks every
// generated authored-span boundary to Go int. A future OpenAPI regression to
// number/float fails at assignment before tests can run.
func compileDocumentSpanOffsetTypes(report DocumentAuditReport) {
	var wholeStart int = report.Wholes[0].Start
	var wholeEnd int = report.Wholes[0].End
	var partStart int = report.Parts[0].Start
	var partEnd int = report.Parts[0].End
	_, _, _, _ = wholeStart, wholeEnd, partStart, partEnd
}

func TestDocumentAuditSpanOffsetsAreIntegers(t *testing.T) {
	report := reflect.TypeOf(DocumentAuditReport{})
	for _, collection := range []string{"Wholes", "Parts"} {
		field, ok := report.FieldByName(collection)
		if !ok {
			t.Fatalf("DocumentAuditReport.%s is missing", collection)
		}
		for _, boundary := range []string{"Start", "End"} {
			spanField, ok := field.Type.Elem().FieldByName(boundary)
			if !ok {
				t.Fatalf("DocumentAuditReport.%s[].%s is missing", collection, boundary)
			}
			if spanField.Type.Kind() != reflect.Int {
				t.Errorf("DocumentAuditReport.%s[].%s is %s, want int", collection, boundary, spanField.Type)
			}
		}
	}
}
