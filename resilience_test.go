package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
	"github.com/joakimcarlsson/model-sync/store"
)

// stub is a source yielding whatever it is told to, so the rules around a
// failing source can be covered without a network or a parser.
type stub struct {
	id     string
	models []catalog.Model
	err    error
}

func (s stub) ID() string   { return s.id }
func (s stub) Name() string { return s.id }

func (s stub) Fetch(context.Context) ([]catalog.Document, error) {
	return []catalog.Document{{URL: "https://" + s.id, Body: []byte("x")}}, nil
}

func (s stub) Parse([]catalog.Document) ([]catalog.Model, error) {
	return s.models, s.err
}

// TestSyncOneSourceFailing covers the rule that one vendor moving a page must
// not cost the others their refresh: the failure is reported, the sources after
// it still write, and the failing one leaves no tree behind.
func TestSyncOneSourceFailing(t *testing.T) {
	dir := t.TempDir()
	good := stub{
		id:     "good",
		models: []catalog.Model{{ID: "m", Provider: "good"}},
	}
	broken := stub{id: "broken"}
	after := stub{
		id:     "after",
		models: []catalog.Model{{ID: "n", Provider: "after"}},
	}
	var failures []error
	for _, source := range []catalog.Source{good, broken, after} {
		if err := sync(context.Background(), dir, source); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) != 1 {
		t.Fatalf("got %d failures, want 1: %v", len(failures), failures)
	}
	if errors.Join(failures...) == nil {
		t.Error("failures did not join into an error")
	}
	for _, id := range []string{"good", "after"} {
		_, err := os.Stat(filepath.Join(dir, id, "provider.json"))
		if err != nil {
			t.Errorf("%s: not written: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "broken")); !os.IsNotExist(err) {
		t.Errorf("the failing source wrote a tree: %v", err)
	}
	cat, err := store.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Count() != 2 {
		t.Errorf("got %d models in the aggregate, want 2", cat.Count())
	}
}

// TestSyncOutOfTimeWritesNothing covers the rule separating a lost document from
// a lost deadline. A source that parsed models is still not written once the
// budget has expired, because what it parsed is however much of the vendor's
// documentation the clock allowed rather than what the vendor publishes.
func TestSyncOutOfTimeWritesNothing(t *testing.T) {
	dir := t.TempDir()
	source := stub{
		id:     "partial",
		models: []catalog.Model{{ID: "m", Provider: "partial"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sync(ctx, dir, source); err == nil {
		t.Error("an expired budget was reported as a success")
	}
	if _, err := os.Stat(filepath.Join(dir, "partial")); !os.IsNotExist(err) {
		t.Errorf("wrote a tree out of time: %v", err)
	}
	if err := sync(context.Background(), dir, source); err != nil {
		t.Fatalf("the same source failed with time to spare: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "partial")); err != nil {
		t.Errorf("not written with time to spare: %v", err)
	}
}
