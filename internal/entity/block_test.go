package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewListWithCorrectSizeAndFreeCount(t *testing.T) {
	wantedByteSize := 100
	wantedFreeSize := wantedByteSize * 8

	newList := NewList(wantedByteSize)

	require.Equal(t, wantedByteSize, len(newList.List))
	require.Equal(t, wantedFreeSize, newList.Free)
}

func TestRevokeIndexInList(t *testing.T) {
	byteListSize := 2

	wantedList := make([]byte, byteListSize)
	for i := range wantedList {
		if i == 0 {
			wantedList[i] = 128
		} else {
			wantedList[i] = 0
		}
	}

	list := NewList(byteListSize)
	list.RevokeAtIndex(7)

	require.Equal(t, wantedList, list.List)
}

func TestGetNextFreeIndexAndReduceFreeCount(t *testing.T) {
	byteListSize := 10
	maxIndex := (byteListSize * 8) - 1

	list := NewList(byteListSize)
	for i := 0; i < maxIndex; i++ {
		_, err := list.AllocateNextFreeIndex()
		if err != nil {
			t.Errorf("unexpected error occured: %v", err)
		}
	}

	wantedIndex := maxIndex
	wantedFreeCount := 0
	index, err := list.AllocateNextFreeIndex()
	if err != nil {
		t.Errorf("unexpected error occured: %v", err)
	}

	require.Equal(t, wantedIndex, index)
	require.Equal(t, wantedFreeCount, list.Free)
}

func TestTryToAllocateNewIndexInFullyAllocatedList(t *testing.T) {
	byteListSize := 10
	maxIndex := (byteListSize * 8) - 1

	list := NewList(byteListSize)
	for i := 0; i <= maxIndex; i++ {
		_, err := list.AllocateNextFreeIndex()
		if err != nil {
			t.Errorf("unexpected error occured: %v", err)
		}
	}

	_, err := list.AllocateNextFreeIndex()
	require.Error(t, ErrFullyAllocated, err)
}

func TestRevoked(t *testing.T) {
	byteListSize := 2
	list := NewList(byteListSize)
	idx, _ := list.AllocateNextFreeIndex()

	list.RevokeAtIndex(idx)
	b := list.CheckBitAtIndex(idx)

	if !b {
		t.Error()
	}

	b = list.CheckBitAtIndex(2)

	if b {
		t.Error()
	}
}
