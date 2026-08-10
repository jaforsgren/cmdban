package azuredevops

import (
	"reflect"
	"testing"
)

func TestDedupeIntsRemovesDuplicatesPreservingOrder(t *testing.T) {
	got := dedupeInts([]int{1, 2, 2, 3, 1, 4})
	want := []int{1, 2, 3, 4}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedupeInts() = %v, want %v", got, want)
	}
}

func TestDedupeIntsEmpty(t *testing.T) {
	got := dedupeInts(nil)
	if len(got) != 0 {
		t.Fatalf("dedupeInts(nil) = %v, want empty", got)
	}
}
