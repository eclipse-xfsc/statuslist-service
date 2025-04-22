package entity

import "fmt"

type StatusData struct {
	Index     int    `json:"index"`
	StatusUrl string `json:"statusUrl"`
}

func NewStatusData(index int, listId int) *StatusData {
	return &StatusData{
		Index:     index,
		StatusUrl: "/status/" + fmt.Sprintf("%d", listId),
	}
}
