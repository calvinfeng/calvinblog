package httpservice

import (
	"github.com/calvinfeng/playground/practice"
	"github.com/google/uuid"
)

type PracticeLogEntryListJSONResponse struct {
	Count   int                  `json:"count"`
	Results []*practice.LogEntry `json:"results"`
	More    bool                 `json:"more"`
}

type PracticeLogLabelListJSONResponse struct {
	Count   int                  `json:"count"`
	Results []*practice.LogLabel `json:"results"`
}

type IDResponse struct {
	ID uuid.UUID `json:"id"`
}
