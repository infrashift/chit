package model

import "testing"

func TestThread_IsValid(t *testing.T) {
	th := &Thread{PostID: NewID(), ChannelID: NewID()}
	if err := th.IsValid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	th.PostID = "bad"
	if err := th.IsValid(); err == nil {
		t.Fatal("expected error for invalid PostID")
	}

	th.PostID = NewID()
	th.ChannelID = "bad"
	if err := th.IsValid(); err == nil {
		t.Fatal("expected error for invalid ChannelID")
	}
}

func TestThreadMembership_IsValid(t *testing.T) {
	tm := &ThreadMembership{PostID: NewID(), UserID: NewID()}
	if err := tm.IsValid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	tm.PostID = "bad"
	if err := tm.IsValid(); err == nil {
		t.Fatal("expected error for invalid PostID")
	}

	tm.PostID = NewID()
	tm.UserID = "bad"
	if err := tm.IsValid(); err == nil {
		t.Fatal("expected error for invalid UserID")
	}
}
