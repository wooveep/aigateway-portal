package portal

import (
	"reflect"
	"testing"
)

func TestCollectManagedDescendants(t *testing.T) {
	t.Parallel()

	rows := []accountHierarchyRow{
		{ConsumerName: "alice", ParentConsumerName: ""},
		{ConsumerName: "bob", ParentConsumerName: "alice"},
		{ConsumerName: "carol", ParentConsumerName: "bob"},
		{ConsumerName: "dave", ParentConsumerName: "alice"},
		{ConsumerName: "eve", ParentConsumerName: "dave"},
		{ConsumerName: "frank", ParentConsumerName: "other"},
	}

	got := collectManagedDescendants(rows, "alice")
	want := []string{"bob", "dave", "carol", "eve"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectManagedDescendants() = %v, want %v", got, want)
	}
}

func TestCollectManagedDescendantsSkipsCycles(t *testing.T) {
	t.Parallel()

	rows := []accountHierarchyRow{
		{ConsumerName: "alice", ParentConsumerName: "carol"},
		{ConsumerName: "bob", ParentConsumerName: "alice"},
		{ConsumerName: "carol", ParentConsumerName: "bob"},
	}

	got := collectManagedDescendants(rows, "alice")
	want := []string{"bob", "carol"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectManagedDescendants() = %v, want %v", got, want)
	}
}
