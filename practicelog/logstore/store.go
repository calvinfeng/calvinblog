package logstore

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/calvinfeng/playground/practicelog"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func New(db *sqlx.DB) practicelog.Store {
	return &store{db: db}
}

type store struct {
	db *sqlx.DB
}

func (s *store) SelectLogEntries() ([]*practicelog.Entry, error) {
	query := squirrel.Select("*").From(PracticeLogEntryTable)

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows := make([]*DBPracticeLogEntry, 0)
	if err = s.db.Select(&rows, statement, args...); err != nil {
		return nil, err
	}

	entries := make([]*practicelog.Entry, 0)
	for _, row := range rows {
		entries = append(entries, row.toModel())
	}

	// This join can become expensive eventually.
	query = squirrel.
		Select("entry_id", "label_id", "parent_id", "name").
		From(PracticeLogLabelTable).
		LeftJoin(
			fmt.Sprintf("%s ON id = label_id", AssociationPracticeLogEntryLabelTable))

	statement, args, err = query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	labelRows := make([]*DBReadOnlyPracticeLogLabel, 0)
	if err = s.db.Select(&labelRows, statement, args...); err != nil {
		return nil, err
	}

	entryToLabels := make(map[uuid.UUID]map[uuid.UUID]*practicelog.Label)
	for _, lbl := range labelRows {
		if _, ok := entryToLabels[lbl.EntryID]; !ok {
			entryToLabels[lbl.EntryID] = make(map[uuid.UUID]*practicelog.Label)
		}
		entryToLabels[lbl.EntryID][lbl.ID] = &practicelog.Label{
			ID:       lbl.ID,
			ParentID: lbl.ParentID,
			Name:     lbl.Name,
		}
	}

	for _, entry := range entries {
		labels, ok := entryToLabels[entry.ID]
		if !ok {
			continue
		}

		entry.Labels = make([]*practicelog.Label, 0, len(labels))
		for _, label := range labels {
			entry.Labels = append(entry.Labels, label)
		}
	}

	return entries, nil
}

func (s *store) SelectLogLabels() ([]*practicelog.Label, error) {
	query := squirrel.Select("*").From(PracticeLogLabelTable)

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows := make([]*DBPracticeLogLabel, 0)
	if err = s.db.Select(&rows, statement, args...); err != nil {
		return nil, err
	}

	labels := make([]*practicelog.Label, 0)
	for _, row := range rows {
		labels = append(labels, row.toModel())
	}

	return labels, nil
}

func (s *store) BatchInsertLogLabels(labels ...*practicelog.Label) (int64, error) {
	query := squirrel.Insert(PracticeLogLabelTable).
		Columns("id", "parent_id", "name")

	for _, label := range labels {
		label.ID = uuid.New()
		row := new(DBPracticeLogLabel).fromModel(label)
		if row.ParentID == uuid.Nil {
			query = query.Values(row.ID, nil, row.Name)
		} else {
			query = query.Values(row.ID, row.ParentID, row.Name)
		}
	}

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}

	res, err := s.db.Exec(statement, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *store) BatchInsertLogEntries(entries ...*practicelog.Entry) (int64, error) {
	entryQuery := squirrel.Insert(PracticeLogEntryTable).
		Columns("id", "user_id", "date", "duration", "title", "note")

	joinQuery := squirrel.Insert(AssociationPracticeLogEntryLabelTable).
		Columns("association_id", "entry_id", "label_id")

	for _, entry := range entries {
		entry.ID = uuid.New()
		row := new(DBPracticeLogEntry).fromModel(entry)
		entryQuery = entryQuery.Values(
			row.ID, row.UserID, row.Date, row.Duration, row.Title, row.Note)

		for _, label := range entry.Labels {
			joinQuery = joinQuery.Values(uuid.New(), entry.ID, label.ID)
		}
	}

	entryInsertStmt, entryArgs, err := entryQuery.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}

	joinInsertStmt, joinArgs, err := joinQuery.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return 0, errors.Wrap(err, "failed to begin transaction")
	}

	res, err := tx.Exec(entryInsertStmt, entryArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query %w, now rollback err=%v", err, tx.Rollback())
	}

	_, err = tx.Exec(joinInsertStmt, joinArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query %w, now rollback err=%v", err, tx.Rollback())
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction %w, now rollback err=%v", err, tx.Rollback())
	}

	return res.RowsAffected()
}
