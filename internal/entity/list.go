package entity

import "fmt"

var ErrFullyAllocated = fmt.Errorf("list is already fully allocated")

type List struct {
	ListId int
	List   []byte
	Free   int
}

func NewList(listSizeInBytes int) *List {
	listBitSize := listSizeInBytes * 8
	newBinaryList := make([]byte, listSizeInBytes)

	for i := range newBinaryList {
		newBinaryList[i] = 0
	}

	return &List{
		ListId: 0,
		List:   newBinaryList,
		Free:   listBitSize,
	}
}

func bitMask(index int) byte {
	return byte(1 << (7 - (index % 8)))
}

func (l *List) RevokeAtIndex(index int) {
	byteIndex := index / 8
	l.List[byteIndex] |= bitMask(index)
}

func (l *List) CheckBitAtIndex(index int) bool {
	byteIndex := index / 8
	return l.List[byteIndex]&bitMask(index) != 0
}

func (b *List) AllocateNextFreeIndex() (index int, err error) {
	if b.Free > 0 {
		index = (len(b.List) * 8) - b.Free
		b.Free--
	} else {
		err = ErrFullyAllocated
	}

	return index, err
}
