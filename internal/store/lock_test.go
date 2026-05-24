package store

import "testing"

func TestLockExcludesSecondHolder(t *testing.T) {
	s := Open(t.TempDir())
	release, err := s.Lock("run1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.tryLock("run1"); err == nil {
		t.Fatal("second lock should fail while first is held")
	}
	release()
	release2, err := s.Lock("run1")
	if err != nil {
		t.Fatal("lock should succeed after release")
	}
	release2()
}
