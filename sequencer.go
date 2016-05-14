package seq

import "sort"

// -----------------------------------------------------------------------------

// ID is a 64bits wide unsigned identifier.
type ID uint64

// IDSlice implements `sort.Interface` for slices of `ID`s.`
type IDSlice []ID

func (ids IDSlice) Len() int           { return len(ids) }
func (ids IDSlice) Less(i, j int) bool { return ids[i] < ids[j] }
func (ids IDSlice) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }
func (ids IDSlice) Sort() IDSlice      { sort.Sort(ids); return ids }

// IDStream is a read-only channel of `ID`s.
//
// An `IDStream` should always starts at 1.
type IDStream <-chan ID

// Next is a simple helper to get the next `ID` from an `IDStream`.
//
// It can be helpful if you cannot simply range on the stream for some reason.
//
// `Next` blocks if no `ID` is available, and returns `0` if the stream is
// already closed.
func (ids IDStream) Next() ID {
	select {
	case id := <-ids:
		return id
	}
}

// -----------------------------------------------------------------------------

// A Sequencer exposes methods to fetch sequential `ID` streams.
type Sequencer interface {
	// GetStream returns the `IDStream` associated with the `Sequencer`.
	GetStream() IDStream
	// Close stops the `Sequencer` and cleans associated resources.
	Close() error
}
