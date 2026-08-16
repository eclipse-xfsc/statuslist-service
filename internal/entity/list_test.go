package entity

import "testing"

func TestListBitOrder_IsMSBFirstWithinByte(t *testing.T) {
	list := List{
		List: make([]byte, 1),
	}

	list.RevokeAtIndex(0)

	if list.List[0] != 0x80 {
		t.Fatalf("index 0 must set MSB, got byte %08b", list.List[0])
	}

	if !list.CheckBitAtIndex(0) {
		t.Fatal("index 0 should be revoked")
	}

	if list.CheckBitAtIndex(7) {
		t.Fatal("index 7 should not be revoked")
	}
}

func TestListBitOrder_Index7SetsLSB(t *testing.T) {
	list := List{
		List: make([]byte, 1),
	}

	list.RevokeAtIndex(7)

	if list.List[0] != 0x01 {
		t.Fatalf("index 7 must set LSB, got byte %08b", list.List[0])
	}

	if !list.CheckBitAtIndex(7) {
		t.Fatal("index 7 should be revoked")
	}

	if list.CheckBitAtIndex(0) {
		t.Fatal("index 0 should not be revoked")
	}
}
