package assert

import "testing"

func TestThatTrueDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("That(true, ...) panicked: %v", r)
		}
	}()
	That(true, "true condition")
}

func TestThatFalsePanicsWithMessage(t *testing.T) {
	defer func() {
		got, ok := recover().(string)
		if !ok {
			t.Fatalf("recover() did not return a string")
		}
		const want = "chit: assertion failed: bad state"
		if got != want {
			t.Fatalf("panic message = %q, want %q", got, want)
		}
	}()
	That(false, "bad state")
	t.Fatalf("That(false, ...) did not panic")
}
