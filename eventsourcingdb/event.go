package eventsourcingdb

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrHashVerificationFailed reports that the hash of an event does not match
// the event, so the event was changed after the server wrote it. VerifyHash
// and VerifySignature return it in that case. Use errors.Is to check for it,
// which also finds it if it is wrapped:
//
//	err := event.VerifyHash()
//	if errors.Is(err, eventsourcingdb.ErrHashVerificationFailed) {
//		// ...
//	}
var ErrHashVerificationFailed = errors.New("hash verification failed")

// ErrSignatureVerificationFailed reports that the signature of an event does
// not match its hash and the verification key, so the event was either not
// signed with the signing key that belongs to the verification key, or changed
// after it was signed. VerifySignature returns it in that case. Use errors.Is
// to check for it, which also finds it if it is wrapped:
//
//	err := event.VerifySignature(verificationKey)
//	if errors.Is(err, eventsourcingdb.ErrSignatureVerificationFailed) {
//		// ...
//	}
var ErrSignatureVerificationFailed = errors.New("signature verification failed")

// ErrUnsupportedHashVersion reports that the hash of an event is of a version
// that this client does not know, so it can not tell whether the hash matches
// the event. Such a hash comes from a newer EventSourcingDB, and is not a sign
// that the event was changed. VerifyHash and VerifySignature return it in that
// case, wrapped in an error that names the version, e.g. "unsupported hash
// version 'v3'". Use errors.Is to check for it:
//
//	err := event.VerifyHash()
//	if errors.Is(err, eventsourcingdb.ErrUnsupportedHashVersion) {
//		// ...
//	}
var ErrUnsupportedHashVersion = errors.New("unsupported hash version")

// hashPrefix starts every hash that names its version. A hash without it is of
// version 1, which predates the prefix.
const hashPrefix = "esdb:hash:"

// hashVersion2Prefix starts every hash of version 2.
const hashVersion2Prefix = hashPrefix + "v2:"

// missingField stands for a field that an event does not have in the input of
// a hash of version 2.
const missingField = "-"

type Event struct {
	SpecVersion     string
	ID              string
	Time            time.Time
	Source          string
	Subject         string
	Type            string
	DataContentType string
	Data            json.RawMessage
	Hash            string
	PredecessorHash string
	TraceParent     *string
	TraceState      *string
	Signature       *string
}

// VerifyHash recomputes the hash of the event from its fields, and compares it
// to the hash the event carries. It knows both versions of the hash: version 2,
// which starts with "esdb:hash:v2:" and also covers the trace context, and
// version 1, which has no prefix. EventSourcingDB writes version 2 since 1.3,
// and events written before keep their hash of version 1.
//
// It returns ErrHashVerificationFailed if the hashes differ, and an error that
// wraps ErrUnsupportedHashVersion if the hash is of a version it does not know.
func (event Event) VerifyHash() error {
	var hash string

	switch {
	case strings.HasPrefix(event.Hash, hashVersion2Prefix):
		hash = event.hashOfVersion2()
	case strings.HasPrefix(event.Hash, hashPrefix):
		return fmt.Errorf("%w '%s'", ErrUnsupportedHashVersion, versionOfHash(event.Hash))
	default:
		hash = event.hashOfVersion1()
	}

	if hash != event.Hash {
		return ErrHashVerificationFailed
	}
	return nil
}

// hashOfVersion1 hashes the hash of the metadata and the hash of the data
// together. It covers neither TraceParent nor TraceState.
func (event Event) hashOfVersion1() string {
	metadata := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		event.SpecVersion,
		event.ID,
		event.PredecessorHash,
		event.Time.Format(time.RFC3339Nano),
		event.Source,
		event.Subject,
		event.Type,
		event.DataContentType,
	)

	metadataHash := sha256.Sum256([]byte(metadata))
	metadataHashHex := fmt.Sprintf("%x", metadataHash)

	dataHash := sha256.Sum256(event.Data)
	dataHashHex := fmt.Sprintf("%x", dataHash)

	finalHash := sha256.Sum256([]byte(metadataHashHex + dataHashHex))
	finalHashHex := fmt.Sprintf("%x", finalHash)

	return finalHashHex
}

// hashOfVersion2 hashes the prefix and every field of the event, each on a
// line of its own, without a line feed after the last one. A field that the
// event does not have is written as "-". The data goes in byte for byte as the
// server sent it, which is why Data is a json.RawMessage.
func (event Event) hashOfVersion2() string {
	traceParent := missingField
	if event.TraceParent != nil {
		traceParent = *event.TraceParent
	}

	traceState := missingField
	if event.TraceState != nil {
		traceState = *event.TraceState
	}

	input := strings.Join([]string{
		hashVersion2Prefix,
		event.SpecVersion,
		event.ID,
		event.PredecessorHash,
		event.Time.Format(time.RFC3339Nano),
		event.Source,
		event.Subject,
		event.Type,
		event.DataContentType,
		traceParent,
		traceState,
		string(event.Data),
	}, "\n")

	hash := sha256.Sum256([]byte(input))

	return hashVersion2Prefix + hex.EncodeToString(hash[:])
}

// versionOfHash returns the version that a hash with a prefix names, e.g. "v3"
// for "esdb:hash:v3:...".
func versionOfHash(hash string) string {
	version := strings.TrimPrefix(hash, hashPrefix)
	if index := strings.IndexByte(version, ':'); index >= 0 {
		version = version[:index]
	}

	return version
}

// VerifySignature verifies the hash of the event, as VerifyHash does, and then
// its signature with the given verification key, which belongs to the signing
// key of the server. The signature covers the hash as it is written, including
// its prefix, if it has one.
//
// It returns the error of VerifyHash if verifying the hash fails, and
// ErrSignatureVerificationFailed if the signature does not match.
func (event Event) VerifySignature(verificationKey ed25519.PublicKey) error {
	if event.Signature == nil {
		return errors.New("signature must not be nil")
	}

	err := event.VerifyHash()
	if err != nil {
		return err
	}

	signaturePrefix := "esdb:signature:v1:"

	if !strings.HasPrefix(*event.Signature, signaturePrefix) {
		return fmt.Errorf("signature must start with '%s'", signaturePrefix)
	}

	signature := strings.TrimPrefix(*event.Signature, signaturePrefix)
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return err
	}

	hashBytes := []byte(event.Hash)

	isSignatureValid := ed25519.Verify(verificationKey, hashBytes, signatureBytes)
	if !isSignatureValid {
		return ErrSignatureVerificationFailed
	}
	return nil
}
