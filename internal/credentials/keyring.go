package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/zalando/go-keyring"
)

const (
	// Service is what the keychain keeps sessions under, each under the name
	// of its context.
	Service = "tarhche-cli"

	// MaxEntry is the most an entry of the keychain is given, in bytes: what
	// Windows' Credential Manager takes, the least of the keychains (macOS
	// takes about 3000 bytes, a secret service anything).
	MaxEntry = 2560

	// maxParts bounds how many entries a secret is kept in, so that a
	// keychain that never says an entry is missing cannot keep a deletion
	// going.
	maxParts = 1000

	// partsPrefix leads the first entry of a secret kept in more than one,
	// followed by how many there are and a colon, then the first part.
	partsPrefix = "tarhche-cli-parts:"

	// partSize is how much of a secret an entry holds when it is split,
	// which leaves the first room to say how many entries there are.
	partSize = MaxEntry - len(partsPrefix) - 8
)

// Backend is a keychain, as go-keyring's functions are one: its Get and
// Delete answer keyring.ErrNotFound for an entry that is not there.
type Backend interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// SystemKeyring is the operating system's keychain: the macOS keychain,
// Windows' Credential Manager, or the secret service of a Linux desktop.
type SystemKeyring struct{}

var _ Backend = SystemKeyring{}

// Get is the secret kept for user under service.
func (SystemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

// Set keeps secret for user under service.
func (SystemKeyring) Set(service, user, secret string) error {
	return keyring.Set(service, user, secret)
}

// Delete forgets the secret of user under service.
func (SystemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// Keyring keeps credentials in a keychain, under Service and the name of
// their context. Credentials larger than an entry takes are split across
// entries: the context's own, then "<context>:1", "<context>:2" and so on,
// which no context is called, since a context's name holds no colon.
type Keyring struct {
	backend Backend
	lock    sync.Mutex
}

var _ Store = &Keyring{}

// NewKeyring is a store keeping credentials in backend.
func NewKeyring(backend Backend) *Keyring {
	return &Keyring{backend: backend}
}

// Get is the credentials kept for a context, joined back from every entry
// they were split across, or ErrNotFound.
func (k *Keyring) Get(context string) (Credentials, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	secret, err := k.read(context)
	if err != nil {
		return Credentials{}, err
	}

	var credentials Credentials
	if err := json.Unmarshal([]byte(secret), &credentials); err != nil {
		return Credentials{}, fmt.Errorf("reading the credentials of %q from the keychain: %w", context, err)
	}

	return credentials, nil
}

// Set keeps credentials for a context, split across entries when they are
// larger than one takes, and forgets the entries of what was kept before
// that are left over.
func (k *Keyring) Set(context string, credentials Credentials) error {
	secret, err := json.Marshal(credentials)
	if err != nil {
		return err
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	if err := k.write(context, string(secret)); err != nil {
		return fmt.Errorf("keeping the credentials of %q in the keychain: %w", context, err)
	}

	return nil
}

// Delete forgets the credentials of a context, every entry of them.
func (k *Keyring) Delete(context string) error {
	k.lock.Lock()
	defer k.lock.Unlock()

	if err := k.deleteParts(context, 1); err != nil {
		return fmt.Errorf("removing the credentials of %q from the keychain: %w", context, err)
	}

	if err := k.backend.Delete(Service, context); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("removing the credentials of %q from the keychain: %w", context, err)
	}

	return nil
}

func (k *Keyring) read(context string) (string, error) {
	head, err := k.backend.Get(Service, context)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("reading the credentials of %q from the keychain: %w", context, err)
	}

	count, first, isSplit := partsOf(head)
	if !isSplit {
		return head, nil
	}

	var secret strings.Builder
	secret.WriteString(first)

	for i := 1; i < count; i++ {
		part, err := k.backend.Get(Service, partName(context, i))
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("the credentials of %q are missing part %d of %d in the keychain: log in again", context, i+1, count)
		} else if err != nil {
			return "", fmt.Errorf("reading the credentials of %q from the keychain: %w", context, err)
		}

		secret.WriteString(part)
	}

	return secret.String(), nil
}

func (k *Keyring) write(context, secret string) error {
	if len(secret) <= MaxEntry {
		if err := k.backend.Set(Service, context, secret); err != nil {
			return err
		}

		return k.deleteParts(context, 1)
	}

	parts := split(secret, partSize)
	if len(parts) > maxParts {
		return fmt.Errorf("%d bytes is more than the keychain can be given", len(secret))
	}

	for i := 1; i < len(parts); i++ {
		if err := k.backend.Set(Service, partName(context, i), parts[i]); err != nil {
			return err
		}
	}

	if err := k.backend.Set(Service, context, partsPrefix+strconv.Itoa(len(parts))+":"+parts[0]); err != nil {
		return err
	}

	return k.deleteParts(context, len(parts))
}

// deleteParts forgets the entries of a context's secret from the one
// numbered from on, until one is not there.
func (k *Keyring) deleteParts(context string, from int) error {
	for i := from; i < maxParts; i++ {
		err := k.backend.Delete(Service, partName(context, i))
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
	}

	return nil
}

// partsOf reads the first entry of a secret: how many entries it is kept
// in, and the part this one holds, when it is kept in more than one.
func partsOf(head string) (count int, first string, isSplit bool) {
	rest, found := strings.CutPrefix(head, partsPrefix)
	if !found {
		return 1, head, false
	}

	number, first, found := strings.Cut(rest, ":")
	if !found {
		return 1, head, false
	}

	count, err := strconv.Atoi(number)
	if err != nil || count < 2 || count > maxParts {
		return 1, head, false
	}

	return count, first, true
}

// partName is the entry part i of a context's secret is kept in.
func partName(context string, i int) string {
	return context + ":" + strconv.Itoa(i)
}

// split cuts secret into parts of size bytes at most, never inside a
// character.
func split(secret string, size int) []string {
	var parts []string

	for len(secret) > size {
		cut := size
		for cut > 0 && !utf8.RuneStart(secret[cut]) {
			cut--
		}

		if cut == 0 {
			cut = size
		}

		parts = append(parts, secret[:cut])
		secret = secret[cut:]
	}

	return append(parts, secret)
}
