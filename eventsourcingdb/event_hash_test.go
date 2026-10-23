package eventsourcingdb_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// The fixtures are events as EventSourcingDB sent them, byte for byte: those
// with a hash of version 1 come from 1.2.0, those with a hash of version 2 from
// a preview of 1.3. Each server signed them with the signing key that
// fixtureVerificationKey belongs to. Of each version, one event has a trace
// context and one has none, and the data of the one without needs JSON
// escaping, which the server writes as Go does.
const (
	fixtureVerificationKey = "442d4550d7defa836df7518745557be20f928f724a0c910b9d398815c7588e8d"

	fixtureOfVersion1 = `{"source":"https://library.eventsourcingdb.io","subject":"/books/42","type":"io.eventsourcingdb.library.book-acquired","specversion":"1.0","id":"0","time":"2026-10-18T13:55:30.454736711Z","datacontenttype":"application/json","predecessorhash":"0000000000000000000000000000000000000000000000000000000000000000","data":{"author":"Arthur C. Clarke","isbn":"978-0756906788","note":"\u003ca \u0026 b\u003e \"quoted\"","pages":296,"title":"2001 – A Space Odyssey"},"hash":"1da3650c5816bce5758be964d9f32f7bd9cb539c33e553ec9952fefb82346a68","signature":"esdb:signature:v1:417a8c14bb74c84c9a38b94612eadcaf4e86fa77a7b6154c96d92143ee038018f2493eee5caf1176b6c0ea57f8cc9b154d152c6e4d0cf84cbe97aa48fbdfc90c"}`

	fixtureOfVersion1WithTraceContext = `{"source":"https://library.eventsourcingdb.io","subject":"/books/42","type":"io.eventsourcingdb.library.book-borrowed","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01","tracestate":"rojo=00f067aa0ba902b7,congo=t61rcWkgMzE","specversion":"1.0","id":"1","time":"2026-10-18T13:55:30.454736711Z","datacontenttype":"application/json","predecessorhash":"1da3650c5816bce5758be964d9f32f7bd9cb539c33e553ec9952fefb82346a68","data":{"borrowedBy":"/readers/23"},"hash":"c8f0718e60ba9e74cb2e48dc767b5c8b37088229314a83339c271004f4c47149","signature":"esdb:signature:v1:cff0305398ce7d55cdb9107e7fb1ac8294a0257318b8f3140158652d66bccc56342022719a323d4ea868260fdbbcd7869d08e2cb1fd5e043dada272de1d5a405"}`

	fixtureOfVersion2 = `{"source":"https://library.eventsourcingdb.io","subject":"/books/42","type":"io.eventsourcingdb.library.book-acquired","specversion":"1.0","id":"0","time":"2026-10-18T13:55:32.849857379Z","datacontenttype":"application/json","predecessorhash":"0000000000000000000000000000000000000000000000000000000000000000","data":{"author":"Arthur C. Clarke","isbn":"978-0756906788","note":"\u003ca \u0026 b\u003e \"quoted\"","pages":296,"title":"2001 – A Space Odyssey"},"hash":"esdb:hash:v2:7ef3618cc49755d22c23804cfaf5ebbd47da29fbfcfd2ac17c3c3cb30fc7c33a","signature":"esdb:signature:v1:b2bf31d0efb03a667c33682ceeffe1a588bc0015eac4d77e29309afc02eac276787f2d03984f4b84a0230e062746e997b53da3aee0fb43a59acb433a649ff700"}`

	fixtureOfVersion2WithTraceContext = `{"source":"https://library.eventsourcingdb.io","subject":"/books/42","type":"io.eventsourcingdb.library.book-borrowed","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01","tracestate":"rojo=00f067aa0ba902b7,congo=t61rcWkgMzE","specversion":"1.0","id":"1","time":"2026-10-18T13:55:32.849857379Z","datacontenttype":"application/json","predecessorhash":"esdb:hash:v2:7ef3618cc49755d22c23804cfaf5ebbd47da29fbfcfd2ac17c3c3cb30fc7c33a","data":{"borrowedBy":"/readers/23"},"hash":"esdb:hash:v2:db3605968caafd0bef77fec0625d1e5a969c4588cee6d98d236821ec7f78e2f0","signature":"esdb:signature:v1:f3530aca3886a75dfb54fad8bbd87105019fd64066d53d7cce679e110debcfa8f670f4bd6944cd3b8283c492131f2a5febdd9a6d16c1f6070f1d40ceefb3f001"}`
)

// A caller has to be able to verify every event of a store, whichever version
// of the hash the server wrote it with, and to tell an event that was changed
// from one whose hash is of a version that the client does not know yet.
func TestVerifyHashOfFixtures(t *testing.T) {
	fixtures := map[string]string{
		"of version 1":                      fixtureOfVersion1,
		"of version 1 with a trace context": fixtureOfVersion1WithTraceContext,
		"of version 2":                      fixtureOfVersion2,
		"of version 2 with a trace context": fixtureOfVersion2WithTraceContext,
	}

	t.Run("verifies a hash", func(t *testing.T) {
		for name, fixture := range fixtures {
			t.Run(name, func(t *testing.T) {
				event := eventOf(t, fixture)

				assert.NoError(t, event.VerifyHash())
			})
		}
	})

	t.Run("verifies a signature", func(t *testing.T) {
		for name, fixture := range fixtures {
			t.Run(name, func(t *testing.T) {
				event := eventOf(t, fixture)

				assert.NoError(t, event.VerifySignature(verificationKeyOfFixtures(t)))
			})
		}
	})

	t.Run("returns ErrHashVerificationFailed if a field that the hash covers was changed", func(t *testing.T) {
		changes := map[string]func(event *eventsourcingdb.Event){
			"SpecVersion":     func(event *eventsourcingdb.Event) { event.SpecVersion = "1.1" },
			"ID":              func(event *eventsourcingdb.Event) { event.ID = "2" },
			"PredecessorHash": func(event *eventsourcingdb.Event) { event.PredecessorHash = strings.Repeat("1", 64) },
			"Time":            func(event *eventsourcingdb.Event) { event.Time = event.Time.Add(1) },
			"Source":          func(event *eventsourcingdb.Event) { event.Source = "https://www.eventsourcingdb.io" },
			"Subject":         func(event *eventsourcingdb.Event) { event.Subject = "/books/23" },
			"Type":            func(event *eventsourcingdb.Event) { event.Type = "io.eventsourcingdb.library.book-returned" },
			"DataContentType": func(event *eventsourcingdb.Event) { event.DataContentType = "text/plain" },
			"Data":            func(event *eventsourcingdb.Event) { event.Data = json.RawMessage(`{"borrowedBy":"/readers/42"}`) },
			"Hash":            func(event *eventsourcingdb.Event) { event.Hash = changedHash(event.Hash) },
		}

		for name, fixture := range fixtures {
			for field, change := range changes {
				t.Run(name+", "+field, func(t *testing.T) {
					event := eventOf(t, fixture)
					change(&event)

					err := event.VerifyHash()

					assert.ErrorIs(t, err, eventsourcingdb.ErrHashVerificationFailed)
					assert.EqualError(t, err, "hash verification failed")
				})
			}
		}
	})

	t.Run("returns ErrHashVerificationFailed if the data was serialized again", func(t *testing.T) {
		for name, fixture := range fixtures {
			t.Run(name, func(t *testing.T) {
				event := eventOf(t, fixture)

				var data map[string]any
				require.NoError(t, json.Unmarshal(event.Data, &data))
				serializedAgain, err := json.MarshalIndent(data, "", "  ")
				require.NoError(t, err)
				event.Data = serializedAgain

				assert.ErrorIs(t, event.VerifyHash(), eventsourcingdb.ErrHashVerificationFailed)
			})
		}
	})

	t.Run("covers the trace context in version 2", func(t *testing.T) {
		changes := map[string]func(event *eventsourcingdb.Event){
			"a changed TraceParent": func(event *eventsourcingdb.Event) {
				event.TraceParent = new("00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01")
			},
			"a changed TraceState": func(event *eventsourcingdb.Event) {
				event.TraceState = new("rojo=00f067aa0ba902b7")
			},
			"a missing TraceParent": func(event *eventsourcingdb.Event) { event.TraceParent = nil },
			"a missing TraceState":  func(event *eventsourcingdb.Event) { event.TraceState = nil },
		}

		for change, apply := range changes {
			t.Run(change, func(t *testing.T) {
				event := eventOf(t, fixtureOfVersion2WithTraceContext)
				apply(&event)

				assert.ErrorIs(t, event.VerifyHash(), eventsourcingdb.ErrHashVerificationFailed)
			})
		}

		t.Run("an added trace context", func(t *testing.T) {
			event := eventOf(t, fixtureOfVersion2)
			event.TraceParent = new("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

			assert.ErrorIs(t, event.VerifyHash(), eventsourcingdb.ErrHashVerificationFailed)
		})
	})

	t.Run("does not cover the trace context in version 1", func(t *testing.T) {
		event := eventOf(t, fixtureOfVersion1WithTraceContext)
		event.TraceParent = nil
		event.TraceState = nil

		assert.NoError(t, event.VerifyHash())
	})

	t.Run("does not take a hash of version 1 for one of version 2, or vice versa", func(t *testing.T) {
		withoutPrefix := eventOf(t, fixtureOfVersion2)
		withoutPrefix.Hash = strings.TrimPrefix(withoutPrefix.Hash, "esdb:hash:v2:")

		withPrefix := eventOf(t, fixtureOfVersion1)
		withPrefix.Hash = "esdb:hash:v2:" + withPrefix.Hash

		assert.ErrorIs(t, withoutPrefix.VerifyHash(), eventsourcingdb.ErrHashVerificationFailed)
		assert.ErrorIs(t, withPrefix.VerifyHash(), eventsourcingdb.ErrHashVerificationFailed)
	})

	t.Run("returns ErrUnsupportedHashVersion, naming the version, if the hash is of an unknown version", func(t *testing.T) {
		hashes := map[string]string{
			"esdb:hash:v3:7ef3618cc49755d22c23804cfaf5ebbd47da29fbfcfd2ac17c3c3cb30fc7c33a":  "v3",
			"esdb:hash:v10:7ef3618cc49755d22c23804cfaf5ebbd47da29fbfcfd2ac17c3c3cb30fc7c33a": "v10",
			"esdb:hash:v3": "v3",
		}

		for hash, version := range hashes {
			t.Run(hash, func(t *testing.T) {
				event := eventOf(t, fixtureOfVersion2)
				event.Hash = hash

				err := event.VerifyHash()

				assert.ErrorIs(t, err, eventsourcingdb.ErrUnsupportedHashVersion)
				assert.NotErrorIs(t, err, eventsourcingdb.ErrHashVerificationFailed)
				assert.EqualError(t, err, fmt.Sprintf("unsupported hash version '%s'", version))
			})
		}
	})

	t.Run("verifies the hash before the signature", func(t *testing.T) {
		changed := eventOf(t, fixtureOfVersion2)
		changed.Subject = "/books/23"

		unknown := eventOf(t, fixtureOfVersion2)
		unknown.Hash = "esdb:hash:v3:" + strings.TrimPrefix(unknown.Hash, "esdb:hash:v2:")

		assert.ErrorIs(t, changed.VerifySignature(verificationKeyOfFixtures(t)), eventsourcingdb.ErrHashVerificationFailed)
		assert.ErrorIs(t, unknown.VerifySignature(verificationKeyOfFixtures(t)), eventsourcingdb.ErrUnsupportedHashVersion)
	})

	t.Run("returns ErrSignatureVerificationFailed if the signature does not match", func(t *testing.T) {
		for name, fixture := range fixtures {
			t.Run(name, func(t *testing.T) {
				event := eventOf(t, fixture)
				event.Signature = new(changedHash(*event.Signature))

				err := event.VerifySignature(verificationKeyOfFixtures(t))

				assert.ErrorIs(t, err, eventsourcingdb.ErrSignatureVerificationFailed)
				assert.EqualError(t, err, "signature verification failed")
			})
		}
	})

	t.Run("fails with an error rather than a panic if the verification key is missing or of another size", func(t *testing.T) {
		event := eventOf(t, fixtureOfVersion2)

		for name, verificationKey := range map[string]ed25519.PublicKey{
			"missing":   nil,
			"too short": verificationKeyOfFixtures(t)[:ed25519.PublicKeySize-1],
			"too long":  append(append(ed25519.PublicKey{}, verificationKeyOfFixtures(t)...), 0),
		} {
			t.Run(name, func(t *testing.T) {
				var err error
				assert.NotPanics(t, func() {
					err = event.VerifySignature(verificationKey)
				})
				assert.EqualError(t, err, fmt.Sprintf("verification key must be an Ed25519 public key of %d bytes", ed25519.PublicKeySize))
			})
		}
	})

	t.Run("checks the verification key before anything else", func(t *testing.T) {
		// An event without a signature and with a changed hash would fail either
		// of the later checks, so only a check of the key that comes first
		// reports the key.
		event := eventOf(t, fixtureOfVersion2)
		event.Hash = changedHash(event.Hash)
		event.Signature = nil

		err := event.VerifySignature(nil)

		assert.EqualError(t, err, fmt.Sprintf("verification key must be an Ed25519 public key of %d bytes", ed25519.PublicKeySize))
	})

	t.Run("signs the hash of version 2 with its prefix", func(t *testing.T) {
		// The signature of an event with a hash of version 2 does not match the
		// hash without its prefix, so the prefix is part of what is signed.
		event := eventOf(t, fixtureOfVersion2)
		signature, err := hex.DecodeString(strings.TrimPrefix(*event.Signature, "esdb:signature:v1:"))
		require.NoError(t, err)

		assert.True(t, ed25519.Verify(verificationKeyOfFixtures(t), []byte(event.Hash), signature))
		assert.False(t, ed25519.Verify(verificationKeyOfFixtures(t), []byte(strings.TrimPrefix(event.Hash, "esdb:hash:v2:")), signature))
	})
}

// eventOf decodes an event as the client decodes the events that it reads.
func eventOf(t *testing.T, fixture string) eventsourcingdb.Event {
	t.Helper()

	var event eventsourcingdb.Event
	require.NoError(t, json.Unmarshal([]byte(fixture), &event))

	return event
}

func verificationKeyOfFixtures(t *testing.T) ed25519.PublicKey {
	t.Helper()

	verificationKey, err := hex.DecodeString(fixtureVerificationKey)
	require.NoError(t, err)

	return ed25519.PublicKey(verificationKey)
}

// changedHash changes the last hexadecimal digit of a hash or a signature.
func changedHash(hash string) string {
	last := hash[len(hash)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}

	return hash[:len(hash)-1] + replacement
}
