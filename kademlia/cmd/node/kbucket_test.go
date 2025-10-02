package node

import (
	"sync"
	"testing"
)

// helper: build a Contact with deterministic ID (last byte = n)
func makeContact(n byte) Contact {
	var id [20]byte
	id[19] = n
	return Contact{ID: id}
}

func TestKbucketAdd_PreservesOrder(t *testing.T) {
	var kb Kbucket
	c1, c2, c3 := makeContact(1), makeContact(2), makeContact(3)

	kb.AddToKBucket(c1)
	kb.AddToKBucket(c2)
	kb.AddToKBucket(c3)

	if got, want := len(kb.Contacts), 3; got != want {
		t.Fatalf("len=%d want=%d", got, want)
	}
	if kb.Contacts[0].ID != c1.ID || kb.Contacts[1].ID != c2.ID || kb.Contacts[2].ID != c3.ID {
		t.Fatalf("order not preserved: %+v", kb.Contacts)
	}
}

func TestKbucketRemove_RemovesFirstMiddleLast(t *testing.T) {
	var kb Kbucket
	c1, c2, c3, c4 := makeContact(1), makeContact(2), makeContact(3), makeContact(4)

	// seed
	kb.AddToKBucket(c1)
	kb.AddToKBucket(c2)
	kb.AddToKBucket(c3)
	kb.AddToKBucket(c4)

	// remove first
	if err := kb.RemoveFromKBucket(c1); err != nil {
		t.Fatalf("remove first: %v", err)
	}
	if len(kb.Contacts) != 3 || kb.Contacts[0].ID != c2.ID {
		t.Fatalf("after remove first, got=%v", kb.Contacts)
	}

	// remove middle (current middle is c3)
	if err := kb.RemoveFromKBucket(c3); err != nil {
		t.Fatalf("remove middle: %v", err)
	}
	if len(kb.Contacts) != 2 || kb.Contacts[0].ID != c2.ID || kb.Contacts[1].ID != c4.ID {
		t.Fatalf("after remove middle, got=%v", kb.Contacts)
	}

	// remove last (current last is c4)
	if err := kb.RemoveFromKBucket(c4); err != nil {
		t.Fatalf("remove last: %v", err)
	}
	if len(kb.Contacts) != 1 || kb.Contacts[0].ID != c2.ID {
		t.Fatalf("after remove last, got=%v", kb.Contacts)
	}
}

func TestKbucketRemove_NotFoundAndEmpty(t *testing.T) {
	var kb Kbucket

	// remove from empty
	if err := kb.RemoveFromKBucket(makeContact(9)); err == nil {
		t.Fatalf("expected error removing from empty")
	}

	// remove non-existent
	kb.AddToKBucket(makeContact(1))
	kb.AddToKBucket(makeContact(2))
	if err := kb.RemoveFromKBucket(makeContact(3)); err == nil {
		t.Fatalf("expected error removing non-existent contact")
	}
}

func TestKbucketAdd_ConcurrentSmoke(t *testing.T) {
	var kb Kbucket
	const N = 200

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			kb.AddToKBucket(makeContact(byte(i)))
		}()
	}
	wg.Wait()

	if got := len(kb.Contacts); got != N {
		t.Fatalf("concurrent add len=%d want=%d", got, N)
	}
}
