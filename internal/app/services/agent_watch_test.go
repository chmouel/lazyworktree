package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestAgentWatcherSeesFirstHookEvent(t *testing.T) {
	spool := filepath.Join(t.TempDir(), "missing", "agent-events")
	hooks := NewAgentHookService(spool, nil)
	if err := hooks.EnsureDir(); err != nil {
		t.Fatalf("ensure spool dir: %v", err)
	}

	watcher := NewAgentWatchService([]string{spool}, 0, nil)
	watcher.SpoolRoots = []string{spool}
	started, err := watcher.Start()
	if err != nil || !started {
		t.Fatalf("start watcher: started=%v err=%v", started, err)
	}
	t.Cleanup(watcher.Stop)
	events := watcher.NextEvent()

	if err := WriteAgentHookEvent(spool, models.AgentHookEvent{
		Agent:         models.AgentKindCodex,
		HookEventName: models.AgentHookUserPromptSubmit,
		SessionID:     "first",
		Timestamp:     time.Now(),
	}); err != nil {
		t.Fatalf("write first hook event: %v", err)
	}

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("first hook event did not notify watcher")
	}
}

func TestAgentWatchPlanRefresh(t *testing.T) {
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

	t.Run("throttles bursts but arms one trailing refresh", func(t *testing.T) {
		w := NewAgentWatchService(nil, 600*time.Millisecond, nil)

		if plan := w.PlanRefresh(base); !plan.Now || plan.TrailingIn != 0 {
			t.Fatalf("first event should refresh immediately, got %+v", plan)
		}
		// First throttled event in the window arms a trailing refresh for the
		// remaining window so the final write still renders.
		if plan := w.PlanRefresh(base.Add(100 * time.Millisecond)); plan.Now || plan.TrailingIn != 500*time.Millisecond {
			t.Fatalf("first throttled event should arm a 500ms trailing refresh, got %+v", plan)
		}
		// Further events in the same window neither refresh nor re-arm.
		if plan := w.PlanRefresh(base.Add(200 * time.Millisecond)); plan.Now || plan.TrailingIn != 0 {
			t.Fatalf("subsequent throttled event should do nothing, got %+v", plan)
		}
		if plan := w.PlanRefresh(base.Add(599 * time.Millisecond)); plan.Now || plan.TrailingIn != 0 {
			t.Fatalf("event just inside the window should do nothing, got %+v", plan)
		}
		if plan := w.PlanRefresh(base.Add(600 * time.Millisecond)); !plan.Now {
			t.Fatalf("event at the debounce boundary should refresh, got %+v", plan)
		}
	})

	t.Run("trailing refresh re-arms after firing", func(t *testing.T) {
		w := NewAgentWatchService(nil, 600*time.Millisecond, nil)
		w.PlanRefresh(base)                             // leading refresh, LastRefresh=base
		w.PlanRefresh(base.Add(100 * time.Millisecond)) // arms trailing

		// The scheduled trailing refresh runs at the end of the window.
		w.TrailingRefreshFired(base.Add(600 * time.Millisecond))

		// A later burst can arm a fresh trailing refresh again.
		if plan := w.PlanRefresh(base.Add(700 * time.Millisecond)); plan.Now || plan.TrailingIn != 500*time.Millisecond {
			t.Fatalf("post-firing event should arm a new trailing refresh, got %+v", plan)
		}
	})

	t.Run("debounce <= 0 disables throttling", func(t *testing.T) {
		w := NewAgentWatchService(nil, 0, nil)
		for i := 0; i < 5; i++ {
			if plan := w.PlanRefresh(base); !plan.Now || plan.TrailingIn != 0 {
				t.Fatalf("refresh %d should be immediate when throttling is disabled, got %+v", i, plan)
			}
		}
	})
}
