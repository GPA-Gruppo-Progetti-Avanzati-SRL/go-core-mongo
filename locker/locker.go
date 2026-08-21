// Package locker provides a MongoDB-backed implementation of the neutral
// go-core-app/lock.Locker primitive. It stores TTL lease documents in a
// dedicated collection and relies on an atomic upsert for mutual exclusion, so
// it needs no extra infrastructure beyond the Mongo connection already used by
// the application. It does not depend on gocron.
package locker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/lock"
	coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// DefaultCollection is the MongoDB collection holding lock lease documents.
	DefaultCollection = "scheduler_locks"

	// defaultTTL bounds how long a lease is held before it may be stolen by
	// another owner. The lock is a dispatch-dedup optimization (correctness
	// lives in DB claiming), so a modest TTL with a single attempt is intended.
	defaultTTL = 30 * time.Second

	// defaultRetryDelay is the wait between acquisition attempts when the caller
	// asked to block (Tries > 1) but did not set an explicit RetryDelay.
	defaultRetryDelay = 100 * time.Millisecond
)

type mongoLocker struct {
	coll *mongo.Collection
	ttl  time.Duration
}

// New returns a MongoDB-backed lock.Locker over the given Mongo service, using
// the raw database so the lock collection needs no prior configuration.
func New(s *coremongo.Service) lock.Locker {
	return &mongoLocker{coll: s.Db().Collection(DefaultCollection), ttl: defaultTTL}
}

// Acquire honours the neutral AcquireOption set: without options it makes a
// single atomic upsert attempt (dispatch-dedup); with Tries > 1 it retries on
// contention (RetryDelay between attempts) until it succeeds, the attempts are
// exhausted, or the context is done. Expiry overrides the lease TTL.
func (l *mongoLocker) Acquire(ctx context.Context, key string, opts ...lock.AcquireOption) (lock.Handle, error) {
	cfg := lock.ResolveAcquireConfig(opts...)
	ttl := l.ttl
	if cfg.Expiry > 0 {
		ttl = cfg.Expiry
	}
	tries := max(cfg.Tries, 1)
	delay := cfg.RetryDelay
	if delay <= 0 {
		delay = defaultRetryDelay
	}

	for attempt := 0; ; attempt++ {
		h, err := l.tryAcquire(ctx, key, ttl)
		if err == nil {
			return h, nil
		}
		if !errors.Is(err, lock.ErrNotAcquired) {
			return nil, err
		}
		if attempt+1 >= tries {
			return nil, lock.ErrNotAcquired
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// tryAcquire takes the lock with an atomic upsert. If a non-expired document
// with the same _id already exists the upsert raises a duplicate-key error,
// which we map to ErrNotAcquired; an expired document is stolen in place.
func (l *mongoLocker) tryAcquire(ctx context.Context, key string, ttl time.Duration) (lock.Handle, error) {
	now := time.Now()
	token := bson.NewObjectID().Hex()
	filter := bson.M{"_id": key, "expiresAt": bson.M{"$lte": now}}
	update := bson.M{"$set": bson.M{"owner": token, "expiresAt": now.Add(ttl)}}

	_, err := l.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, lock.ErrNotAcquired
		}
		return nil, fmt.Errorf("mongo lock acquire %q: %w", key, err)
	}
	return &mongoHandle{coll: l.coll, key: key, token: token, ttl: ttl}, nil
}

type mongoHandle struct {
	coll  *mongo.Collection
	key   string
	token string
	ttl   time.Duration
}

// Release deletes the lease only if this owner still holds it (a stolen/expired
// lease deletes nothing, which is fine for a dispatch-dedup lock).
func (h *mongoHandle) Release(ctx context.Context) error {
	if _, err := h.coll.DeleteOne(ctx, bson.M{"_id": h.key, "owner": h.token}); err != nil {
		return fmt.Errorf("mongo lock release %q: %w", h.key, err)
	}
	return nil
}

// Extend renews the lease TTL only if this owner still holds it. A lost lease
// (stolen after expiry, or already released) matches nothing and is surfaced as
// lock.ErrLockLost.
func (h *mongoHandle) Extend(ctx context.Context) error {
	res, err := h.coll.UpdateOne(ctx,
		bson.M{"_id": h.key, "owner": h.token},
		bson.M{"$set": bson.M{"expiresAt": time.Now().Add(h.ttl)}})
	if err != nil {
		return fmt.Errorf("mongo lock extend %q: %w", h.key, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("mongo lock extend %q: %w", h.key, lock.ErrLockLost)
	}
	return nil
}
