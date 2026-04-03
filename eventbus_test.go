package turbine

import (
	"sync"
	"testing"
	"time"
)

func TestEventBusNotifyWake(t *testing.T) {
	eb := newEventBus()
	ch := eb.Wait("wf1::topic1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		eb.Notify("wf1::topic1")
	}()

	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestEventBusNoFalseWake(t *testing.T) {
	eb := newEventBus()
	ch := eb.Wait("wf1::topic1")

	eb.Notify("wf2::topic1")

	select {
	case <-ch:
		t.Fatal("should not have been notified")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEventBusMultipleWaiters(t *testing.T) {
	eb := newEventBus()
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			ch := eb.Wait("wf1::key")
			<-ch
		}()
	}

	time.Sleep(10 * time.Millisecond)
	eb.Notify("wf1::key")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("not all waiters woke up")
	}
}

func TestEventBusRemove(t *testing.T) {
	eb := newEventBus()
	ch := eb.Wait("wf1::key")
	eb.Remove("wf1::key", ch)

	eb.Notify("wf1::key")

	select {
	case <-ch:
		t.Fatal("removed waiter should not be notified")
	case <-time.After(50 * time.Millisecond):
	}
}
