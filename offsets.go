package ezclassify

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// UTF8ByteOffset returns the zero-based UTF-8 byte offset for a zero-based
// rune index in text. The index may equal the rune count to address the
// end-exclusive boundary. Invalid UTF-8 and out-of-range indices return an
// error rather than silently substituting replacement characters.
//
// Each call validates and walks text from the start, so converting many
// positions over one document costs a pass per position. Use
// [UTF8ByteOffsets] to convert a whole set in one sweep.
func UTF8ByteOffset(text string, runeIndex int) (int, error) {
	if !utf8.ValidString(text) {
		return 0, errInvalidUTF8()
	}
	if runeIndex < 0 {
		return 0, errRuneIndexRange(runeIndex)
	}

	index := 0
	for byteOffset := range text {
		if index == runeIndex {
			return byteOffset, nil
		}
		index++
	}
	if index == runeIndex {
		return len(text), nil
	}
	return 0, errRuneIndexRange(runeIndex)
}

// UTF8RuneIndex returns the zero-based rune index for a zero-based UTF-8 byte
// offset in text. The offset may equal len(text) to address the end-exclusive
// boundary. Offsets inside a multi-byte encoding, invalid UTF-8, and
// out-of-range offsets return an error.
//
// Like [UTF8ByteOffset], each call walks text from the start. Use
// [UTF8RuneIndexes] to convert a whole set in one sweep.
func UTF8RuneIndex(text string, byteOffset int) (int, error) {
	if !utf8.ValidString(text) {
		return 0, errInvalidUTF8()
	}
	if byteOffset < 0 || byteOffset > len(text) {
		return 0, errByteOffsetRange(byteOffset)
	}
	if byteOffset < len(text) && !utf8.RuneStart(text[byteOffset]) {
		return 0, errByteOffsetBoundary(byteOffset)
	}
	return utf8.RuneCountInString(text[:byteOffset]), nil
}

// UTF8ByteOffsets converts a set of zero-based rune indices to UTF-8 byte
// offsets in one pass over text, for callers mapping the Start and End of
// every [ScanMatch] in a document back onto their own buffer. The result is
// parallel to runeIndexes, keeping the caller's order and duplicates; the
// indices need not be sorted. The per-index contract is [UTF8ByteOffset]'s,
// and the first index that violates it fails the whole call.
func UTF8ByteOffsets(text string, runeIndexes []int) ([]int, error) {
	if !utf8.ValidString(text) {
		return nil, errInvalidUTF8()
	}
	offsets := make([]int, len(runeIndexes))
	order := ascendingOrder(runeIndexes)

	// One walk resolves every request: the order slice hands them over in
	// ascending order, so the scan never restarts.
	next := 0
	runeIndex := 0
	for byteOffset := range text {
		for next < len(order) && runeIndexes[order[next]] == runeIndex {
			offsets[order[next]] = byteOffset
			next++
		}
		runeIndex++
	}
	for next < len(order) && runeIndexes[order[next]] == runeIndex {
		offsets[order[next]] = len(text)
		next++
	}
	if next < len(order) {
		return nil, errRuneIndexRange(runeIndexes[order[next]])
	}
	return offsets, nil
}

// UTF8RuneIndexes converts a set of zero-based UTF-8 byte offsets to rune
// indices in one pass over text. The result is parallel to byteOffsets,
// keeping the caller's order and duplicates; the offsets need not be sorted.
// The per-offset contract is [UTF8RuneIndex]'s, and the first offset that
// violates it fails the whole call.
func UTF8RuneIndexes(text string, byteOffsets []int) ([]int, error) {
	if !utf8.ValidString(text) {
		return nil, errInvalidUTF8()
	}
	for _, byteOffset := range byteOffsets {
		if byteOffset < 0 || byteOffset > len(text) {
			return nil, errByteOffsetRange(byteOffset)
		}
		if byteOffset < len(text) && !utf8.RuneStart(text[byteOffset]) {
			return nil, errByteOffsetBoundary(byteOffset)
		}
	}
	indexes := make([]int, len(byteOffsets))
	order := ascendingOrder(byteOffsets)

	next := 0
	runeIndex := 0
	for offset := range text {
		for next < len(order) && byteOffsets[order[next]] == offset {
			indexes[order[next]] = runeIndex
			next++
		}
		runeIndex++
	}
	for next < len(order) {
		// Every remaining offset is len(text): the loop above covered each
		// rune boundary below it, and the validation pass rejected the rest.
		indexes[order[next]] = runeIndex
		next++
	}
	return indexes, nil
}

// ascendingOrder returns the positions of values, ordered by the value each
// one holds, so a single forward scan can serve an unsorted request set.
func ascendingOrder(values []int) []int {
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return values[order[a]] < values[order[b]] })
	return order
}

func errInvalidUTF8() error {
	return fmt.Errorf("ezclassify: text is not valid UTF-8")
}

func errRuneIndexRange(runeIndex int) error {
	return fmt.Errorf("ezclassify: rune index %d is out of range", runeIndex)
}

func errByteOffsetRange(byteOffset int) error {
	return fmt.Errorf("ezclassify: byte offset %d is out of range", byteOffset)
}

func errByteOffsetBoundary(byteOffset int) error {
	return fmt.Errorf("ezclassify: byte offset %d is not a rune boundary", byteOffset)
}
