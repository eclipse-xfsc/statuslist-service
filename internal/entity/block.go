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

func (b *List) CheckBitAtIndex(index int) bool {
	if len(b.List) == 0 {
		return false
	}
	byteIndex, bitIndex := index/8, index%8

	return (b.List[byteIndex] & (1 << bitIndex)) != 0

}

func (b *List) RevokeAtIndex(index int) {
	byteIndex, bitIndex := index/8, index%8
	b.List[byteIndex] |= (1 << bitIndex)
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
