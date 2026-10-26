package ezclassify

// exclusiveInputError checks the request's declared input choice without
// interpreting any marking, assertion source or format identifier. A request
// that names neither or both sources, or a format alongside facts, is refused
// before transport as an [InvalidInputError].
func exclusiveInputError(op string, input, facts bool, inputFormat *string) error {
	if input == facts {
		return &InvalidInputError{Op: op, Detail: "input-or-assertions"}
	}
	if facts && inputFormat != nil {
		return &InvalidInputError{Op: op, Detail: "input-format-with-assertions"}
	}
	return nil
}
