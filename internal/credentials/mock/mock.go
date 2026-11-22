// Package mock stands in for where sessions are kept: a store of
// credentials and a keychain, both in memory, either of which fails what a
// test tells it to.
package mock

import (
	"maps"
	"slices"
	"sync"

	"github.com/zalando/go-keyring"

	"github.com/Tarhche/cli/internal/credentials"
)

// Store keeps credentials in memory.
type Store struct {
	// Fail, when it is set, is asked before every operation ("get", "set"
	// or "delete") and fails it with what it answers.
	Fail func(operation, context string) error

	lock     sync.Mutex
	kept     map[string]credentials.Credentials
	setCalls int
}

var _ credentials.Store = &Store{}

// NewStore is a store keeping kept, by context.
func NewStore(kept map[string]credentials.Credentials) *Store {
	s := &Store{kept: make(map[string]credentials.Credentials)}
	maps.Copy(s.kept, kept)

	return s
}

// Get is the credentials kept for a context, or credentials.ErrNotFound.
func (s *Store) Get(context string) (credentials.Credentials, error) {
	if err := s.fail("get", context); err != nil {
		return credentials.Credentials{}, err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	kept, found := s.kept[context]
	if !found {
		return credentials.Credentials{}, credentials.ErrNotFound
	}

	return kept, nil
}

// Set keeps credentials for a context.
func (s *Store) Set(context string, c credentials.Credentials) error {
	if err := s.fail("set", context); err != nil {
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.kept[context] = c
	s.setCalls++

	return nil
}

// Delete forgets the credentials of a context.
func (s *Store) Delete(context string) error {
	if err := s.fail("delete", context); err != nil {
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.kept, context)

	return nil
}

// Sets is how many times credentials were kept.
func (s *Store) Sets() int {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.setCalls
}

func (s *Store) fail(operation, context string) error {
	if s.Fail == nil {
		return nil
	}

	return s.Fail(operation, context)
}

// Keyring is a keychain in memory.
type Keyring struct {
	// MaxSize, when it is not zero, is the most an entry takes, as a
	// keychain refuses more with keyring.ErrSetDataTooBig.
	MaxSize int

	// Fail, when it is set, is asked before every operation ("get", "set"
	// or "delete") and fails it with what it answers.
	Fail func(operation, service, user string) error

	lock    sync.Mutex
	entries map[string]map[string]string
}

var _ credentials.Backend = &Keyring{}

// NewKeyring is an empty keychain taking what Windows' takes at most.
func NewKeyring() *Keyring {
	return &Keyring{MaxSize: credentials.MaxEntry, entries: make(map[string]map[string]string)}
}

// Get is the secret kept for user under service, or keyring.ErrNotFound.
func (k *Keyring) Get(service, user string) (string, error) {
	if err := k.fail("get", service, user); err != nil {
		return "", err
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	secret, found := k.entries[service][user]
	if !found {
		return "", keyring.ErrNotFound
	}

	return secret, nil
}

// Set keeps secret for user under service.
func (k *Keyring) Set(service, user, secret string) error {
	if err := k.fail("set", service, user); err != nil {
		return err
	}

	if k.MaxSize > 0 && len(secret) > k.MaxSize {
		return keyring.ErrSetDataTooBig
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	if k.entries[service] == nil {
		k.entries[service] = make(map[string]string)
	}

	k.entries[service][user] = secret

	return nil
}

// Delete forgets the secret of user under service, or is
// keyring.ErrNotFound.
func (k *Keyring) Delete(service, user string) error {
	if err := k.fail("delete", service, user); err != nil {
		return err
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	if _, found := k.entries[service][user]; !found {
		return keyring.ErrNotFound
	}

	delete(k.entries[service], user)

	return nil
}

// Users are the users something is kept for under service, in order.
func (k *Keyring) Users(service string) []string {
	k.lock.Lock()
	defer k.lock.Unlock()

	return slices.Sorted(maps.Keys(k.entries[service]))
}

func (k *Keyring) fail(operation, service, user string) error {
	if k.Fail == nil {
		return nil
	}

	return k.Fail(operation, service, user)
}
