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

func (event Event) VerifyHash() error {
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

	if finalHashHex != event.Hash {
		return errors.New("hash verification failed")
	}
	return nil
}

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
		return errors.New("signature verification failed")
	}
	return nil
}
