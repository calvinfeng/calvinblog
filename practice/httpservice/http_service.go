package httpservice

import (
	"fmt"
	"github.com/calvinfeng/playground/practice"
	"github.com/calvinfeng/playground/practice/logstore"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"net/http"
)

func New(store practice.LogStore) practice.HTTPService {
	return &service{
		store:    store,
		validate: validator.New(),
	}
}

type service struct {
	store    practice.LogStore
	validate *validator.Validate
}

func (s *service) DeletePracticeLogLabel(c echo.Context) error {
	id, err := uuid.Parse(c.Param("label_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "log label id in path parameter is not a valid uuid ").Error())
	}

	if err := s.store.DeleteLogLabel(&practice.LogLabel{
		ID: id,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to delete entry from database").Error())
	}

	return c.JSON(http.StatusOK, IDResponse{ID: id})
}

func (s *service) DeletePracticeLogEntry(c echo.Context) error {
	id, err := uuid.Parse(c.Param("entry_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "log label id in path parameter is not a valid uuid ").Error())
	}

	if err := s.store.DeleteLogEntry(&practice.LogEntry{
		ID: id,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to delete entry from database").Error())
	}

	return c.JSON(http.StatusOK, IDResponse{ID: id})
}

func (s *service) CreatePracticeLogLabel(c echo.Context) error {
	label := new(practice.LogLabel)
	if err := c.Bind(label); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "failed to parse JSON data").Error())
	}

	if err := s.validate.Struct(label); err != nil {
		fieldErrors := err.(validator.ValidationErrors)
		jsonM := make(map[string]string)
		for _, fieldErr := range fieldErrors {
			field := fieldErr.Field()
			err := fieldErr.(error)
			jsonM[field] = err.Error()
		}
		return echo.NewHTTPError(http.StatusBadRequest, jsonM)
	}

	if _, err := s.store.BatchInsertLogLabels(label); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to insert to database").Error())
	}

	return c.JSON(http.StatusCreated, IDResponse{ID: label.ID})
}

func (s *service) UpdatePracticeLogLabel(c echo.Context) error {
	label := new(practice.LogLabel)
	if err := c.Bind(label); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "failed to parse JSON data").Error())
	}

	if id, err := uuid.Parse(c.Param("label_id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "log label id in path parameter is not a valid uuid ").Error())
	} else {
		if id != label.ID {
			return echo.NewHTTPError(http.StatusBadRequest,
				"payload log label ID does not match with path log label ID")
		}
	}

	if err := s.validate.Struct(label); err != nil {
		fieldErrors := err.(validator.ValidationErrors)
		jsonM := make(map[string]string)
		for _, fieldErr := range fieldErrors {
			field := fieldErr.Field()
			err := fieldErr.(error)
			jsonM[field] = err.Error()
		}
		return echo.NewHTTPError(http.StatusBadRequest, jsonM)
	}

	if err := s.store.UpdateLogLabel(label); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to update log label").Error())
	}

	return c.JSON(http.StatusOK, label)
}

func (s *service) CreatePracticeLogEntry(c echo.Context) error {
	entry := new(practice.LogEntry)
	if err := c.Bind(entry); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "failed to parse JSON data").Error())
	}

	if err := s.validate.Struct(entry); err != nil {
		fieldErrors := err.(validator.ValidationErrors)
		jsonM := make(map[string]string)
		for _, fieldErr := range fieldErrors {
			field := fieldErr.Field()
			err := fieldErr.(error)
			jsonM[field] = err.Error()
		}
		return echo.NewHTTPError(http.StatusBadRequest, jsonM)
	}

	if _, err := s.store.BatchInsertLogEntries(entry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to insert to database").Error())
	}

	return c.JSON(http.StatusCreated, IDResponse{ID: entry.ID})
}

func (s *service) UpdatePracticeLogEntry(c echo.Context) error {
	entry := new(practice.LogEntry)
	if err := c.Bind(entry); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "failed to parse JSON data").Error())
	}

	if id, err := uuid.Parse(c.Param("entry_id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "log entry id in path parameter is not a valid uuid ").Error())
	} else {
		if id != entry.ID {
			return echo.NewHTTPError(http.StatusBadRequest,
				"payload log entry ID does not match with path log entry ID")
		}
	}

	if err := s.validate.Struct(entry); err != nil {
		fieldErrors := err.(validator.ValidationErrors)
		jsonM := make(map[string]string)
		for _, fieldErr := range fieldErrors {
			field := fieldErr.Field()
			err := fieldErr.(error)
			jsonM[field] = err.Error()
		}
		return echo.NewHTTPError(http.StatusBadRequest, jsonM)
	}

	for i, assignment := range entry.Assignments {
		if assignment.Position != i {
			return echo.NewHTTPError(http.StatusBadRequest,
				"assignments must have correct position values, [0, ..., N]")
		}
	}

	if err := s.store.UpdateLogEntry(entry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to update log assignments").Error())
	}

	return c.JSON(http.StatusOK, entry)
}

func (s *service) UpdatePracticeLogAssignments(c echo.Context) error {
	entry := new(practice.LogEntry)
	if err := c.Bind(entry); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			errors.Wrap(err, "failed to parse JSON data").Error())
	}

	for i, assignment := range entry.Assignments {
		if assignment.Position != i {
			return echo.NewHTTPError(http.StatusBadRequest,
				"assignments must have correct position values, [0, ..., N]")
		}
	}

	if err := s.store.UpdateLogAssignments(entry); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "failed to update log assignments").Error())
	}

	entryID := c.Param("entry_id")

	entries, err := s.store.SelectLogEntries(1, 0, logstore.ByID(entryID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Errorf("server failed to query store %w", err))
	}

	return c.JSON(http.StatusOK, entries[0])
}

func (s *service) ListPracticeLogEntries(c echo.Context) error {
	count, err := s.store.CountLogEntries()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			errors.Wrap(err, "server failed to query database").Error())
	}

	limit, offset := getLimitOffsetFromContext(c)

	resp := new(PracticeLogEntryListJSONResponse)
	if count > int(limit)+int(offset) {
		resp.More = true
	}

	entries, err := s.store.SelectLogEntries(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Errorf("server failed to query store %w", err))
	}
	resp.Results = entries
	resp.Count = len(entries)

	return c.JSON(http.StatusOK, resp)
}

func (s *service) ListPracticeLogLabels(c echo.Context) error {
	resp := new(PracticeLogLabelListJSONResponse)
	labels, err := s.store.SelectLogLabels()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Errorf("server failed to query store %w", err))
	}
	resp.Count = len(labels)
	resp.Results = labels
	return c.JSON(http.StatusOK, resp)
}
