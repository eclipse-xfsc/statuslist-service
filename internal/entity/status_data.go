package entity

import "fmt"

type StatusData struct {
	ListId    int    `json:"listid"`
	Index     int    `json:"index"`
	StatusUrl string `json:"statusUrl"`
}

func NewStatusData(index int, listId int) *StatusData {
	return &StatusData{
		ListId:    listId,
		Index:     index,
		StatusUrl: "/status/" + fmt.Sprintf("%d", listId),
	}
}
