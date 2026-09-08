package locker

import (
	"context"
	"strings"
	"testing"

	coremongo "github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-mongo"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/tpm-mongo-common/mongolks"
	"go.uber.org/fx"
)

// unconnectedService reproduces the state of the Mongo service during fx graph
// construction: built, but with Connect still pending in the OnStart hook, so
// Db() returns nil.
func unconnectedService(t *testing.T) *coremongo.Service {
	t.Helper()
	mls, err := mongolks.NewLinkedServiceWithConfig(coremongo.Config{Name: "test"})
	if err != nil {
		t.Fatalf("NewLinkedServiceWithConfig: %v", err)
	}
	svc := &coremongo.Service{LinkedService: mls}
	if svc.Db() != nil {
		t.Fatal("precondition failed: Db() should be nil before Connect")
	}
	return svc
}

// recordingLifecycle collects the hooks a constructor registers, so a test can
// run them explicitly and observe the outcome.
type recordingLifecycle struct{ hooks []fx.Hook }

func (l *recordingLifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }

func (l *recordingLifecycle) start(ctx context.Context) error {
	for _, h := range l.hooks {
		if h.OnStart == nil {
			continue
		}
		if err := h.OnStart(ctx); err != nil {
			return err
		}
	}
	return nil
}

// TestNewDoesNotTouchDb guards the regression that made wiring locker.Module
// alongside coremongo.Module abort the application: New used to resolve the
// collection eagerly via Service.Db(), which is nil until the OnStart hook runs,
// panicking in mongo.newCollection before the app ever started.
func TestNewDoesNotTouchDb(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New panicked on an unconnected service: %v", r)
		}
	}()

	if l := New(unconnectedService(t), &recordingLifecycle{}); l == nil {
		t.Fatal("New returned a nil Locker")
	}
}

// TestNewRegistersStartHook checks that the collection is resolved by the
// lifecycle and not lazily on first use: it is what makes a broken setup stop the
// application at startup instead of at the first scheduler tick.
func TestNewRegistersStartHook(t *testing.T) {
	lc := &recordingLifecycle{}
	New(unconnectedService(t), lc)

	if len(lc.hooks) != 1 {
		t.Fatalf("hooks registered = %d, want 1", len(lc.hooks))
	}
	if lc.hooks[0].OnStart == nil {
		t.Fatal("the registered hook has no OnStart")
	}
}

// TestStartOnUnconnectedServiceErrors checks that a service that never connected
// makes the start fail with a readable error, rather than crashing the process.
func TestStartOnUnconnectedServiceErrors(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the start hook panicked on an unconnected service: %v", r)
		}
	}()

	lc := &recordingLifecycle{}
	New(unconnectedService(t), lc)

	err := lc.start(context.Background())
	if err == nil {
		t.Fatal("expected an error starting on an unconnected service, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestAcquireBeforeStartErrors checks that using the lock before the lifecycle
// started it is reported as an error rather than dereferencing a nil collection.
func TestAcquireBeforeStartErrors(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Acquire panicked before start: %v", r)
		}
	}()

	l := New(unconnectedService(t), &recordingLifecycle{})

	_, err := l.Acquire(context.Background(), "some-key")
	if err == nil {
		t.Fatal("expected an error acquiring before start, got nil")
	}
	if !strings.Contains(err.Error(), "not been started") {
		t.Errorf("unexpected error: %v", err)
	}
}
