// Package locker provides a MongoDB-backed implementation of the neutral
// go-core-app/lock.Locker primitive. It stores TTL lease documents in a
// dedicated collection and relies on an atomic upsert for mutual exclusion, so
// it needs no extra infrastructure beyond the Mongo connection already used by
// the application. It does not depend on gocron.
package locker

import (
	"context"
	"fmt"
	"time"

	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/lock"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
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
)

type mongoLocker struct {
	coll *mongo.Collection
	ttl  time.Duration
}

// New returns a MongoDB-backed lock.Locker over the given linked service, using
// the raw database so the lock collection needs no prior configuration.
func New(ls *mongolks.LinkedService) lock.Locker {
	return &mongoLocker{coll: ls.Db().Collection(DefaultCollection), ttl: defaultTTL}
}

// Acquire takes the lock with an atomic upsert. If a non-expired document with
// the same _id already exists the upsert raises a duplicate-key error, which we
// map to ErrNotAcquired; an expired document is stolen in place.
func (l *mongoLocker) Acquire(ctx context.Context, key string) (lock.Handle, error) {
	now := time.Now()
	token := bson.NewObjectID().Hex()
	filter := bson.M{"_id": key, "expiresAt": bson.M{"$lte": now}}
	update := bson.M{"$set": bson.M{"owner": token, "expiresAt": now.Add(l.ttl)}}

	_, err := l.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, lock.ErrNotAcquired
		}
		return nil, fmt.Errorf("mongo lock acquire %q: %w", key, err)
	}
	return &mongoHandle{coll: l.coll, key: key, token: token}, nil
}

type mongoHandle struct {
	coll  *mongo.Collection
	key   string
	token string
}

// Release deletes the lease only if this owner still holds it (a stolen/expired
// lease deletes nothing, which is fine for a dispatch-dedup lock).
func (h *mongoHandle) Release(ctx context.Context) error {
	if _, err := h.coll.DeleteOne(ctx, bson.M{"_id": h.key, "owner": h.token}); err != nil {
		return fmt.Errorf("mongo lock release %q: %w", h.key, err)
	}
	return nil
}
