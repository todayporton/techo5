package activity

// fork: Ended (ended_fork.go).

import "testing"

func TestEndedFiresOnceWithWhy(t *testing.T) {
	caught(t) // Home Assistant's events go to the test's sender, not nowhere
	var got []Outcome
	cancel := Ended.Listen(func(o Outcome) { got = append(got, o) })
	defer cancel()

	turn := Get().Begin(1, "hey jarvis", WakeWord)
	turn.Fails("stt-failed")
	turn.Ends(Failed)
	turn.Ends(Answered)

	if len(got) != 1 {
		t.Fatalf("Ended fired %d times, want once", len(got))
	}
	if o := got[0]; o.How != Failed || o.Failure != "stt-failed" || o.At.IsZero() {
		t.Errorf("outcome %+v", o)
	}

	got = nil
	turn = Get().Begin(1, "hey jarvis", WakeWord)
	turn.Cancels(Stopped)
	turn.Ends(Canceled)
	if len(got) != 1 || got[0].How != Canceled || got[0].Reason != Stopped {
		t.Errorf("canceled outcome %+v", got)
	}
}
