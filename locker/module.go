package locker

import (
	core "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app/lock"
)

// Module registers the MongoDB-backed lock.Locker in the fx application. It is
// modes-only: it consumes the *coremongo.Service provided by coremongo.Module,
// so no extra config is required.
//
//	batch.Module(&cfg.Batch, batch.WithLocker(locker.Module), ...)
func Module(modes ...string) {
	core.ProvideAs[lock.Locker](New, modes...)
}
