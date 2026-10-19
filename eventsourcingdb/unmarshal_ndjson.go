package eventsourcingdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
)

// v1Line is a line of a stream of the v1 API, whose payload depends on its
// type.
type v1Line struct {
	Type    string
	Payload json.RawMessage
}

// envelope is an item of the v2 API, in a stream or as a single answer, whose
// data depends on its type.
type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// unmarshalNDJSON reads the lines of a stream of newline-delimited JSON, and
// decodes each of them as a T.
func unmarshalNDJSON[T any](ctx context.Context, r io.Reader) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		// We decode directly from the reader using a json.Decoder instead
		// of reading whole lines with a bufio.Scanner. The scanner enforces
		// a fixed maximum token size, so a single large event would exceed
		// the buffer and fail with bufio.ErrTooLong. The actual byte size is
		// hard to predict, because JSON escaping (quotes, control characters,
		// Unicode) can inflate a payload well beyond its raw size. The
		// json.Decoder has no such line-length limit, decouples the client
		// from the server's payload size limit, and avoids the intermediate
		// copy that scanner.Text() would create. It reads successive JSON
		// values from the stream, transparently skipping the newlines that
		// separate the NDJSON lines.
		//
		// This deliberately imposes no upper bound on the size of a single
		// value. We rely on the fact that the client talks to a trusted,
		// authenticated EventSourcingDB server that already bounds payload
		// sizes on its side; a malformed or hostile server could otherwise
		// make the decoder buffer an arbitrarily large value.
		decoder := json.NewDecoder(r)

		for {
			select {
			case <-ctx.Done():
				yield(*new(T), ctx.Err())
				return
			default:
				// From a technical point of view, this default block
				// is not necessary. Actually, the entire select is
				// only there to avoid blocking, and a select with just
				// a single case would be sufficient, as it does not
				// block anyway. However, this is a common pattern in
				// Go, and it is used here to make the code more
				// readable.
			}

			var line T
			err := decoder.Decode(&line)
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				// A decoding error leaves the decoder's position within the
				// stream unrecoverable, so we report the error and stop
				// instead of trying to resynchronize on the next line.
				yield(*new(T), err)
				return
			}

			if !yield(line, nil) {
				return
			}
		}
	}
}
