package auth

import (
	"testing"
	"time"
)

func TestLimiterBlocksAfterRepeatedFailures(t *testing.T) {
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	l := newLimiter(3, time.Minute, func() time.Time { return clock })

	for i := range 3 {
		if wait := l.blocked("ada|1.2.3.4"); wait != 0 {
			t.Fatalf("attempt %d blocked early", i)
		}
		l.fail("ada|1.2.3.4")
	}
	if wait := l.blocked("ada|1.2.3.4"); wait <= 0 || wait > time.Minute {
		t.Errorf("blocked = %v, want up to a minute", wait)
	}
	if l.blocked("ada|5.6.7.8") != 0 || l.blocked("bob|1.2.3.4") != 0 {
		t.Error("other emails and addresses must not be blocked")
	}

	clock = clock.Add(61 * time.Second)
	if l.blocked("ada|1.2.3.4") != 0 {
		t.Error("the block must lift after the window")
	}
}

func TestLimiterResetsOnSuccess(t *testing.T) {
	l := newLimiter(2, time.Minute, time.Now)
	l.fail("k")
	l.succeed("k")
	l.fail("k")
	if l.blocked("k") != 0 {
		t.Error("a success must reset the failure count")
	}
}
