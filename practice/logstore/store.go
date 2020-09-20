package logstore

import (
	"database/sql"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/calvinfeng/playground/practice"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func New(db *sqlx.DB) practice.LogStore {
	return &store{db: db}
}

type store struct {
	db *sqlx.DB
}

func (s *store) CountLogEntries(filters ...practice.SQLFilter) (int, error) {
	query := squirrel.Select("COUNT(*)").From(PracticeLogEntryTable)

	eqCondition := make(squirrel.Eq)
	for _, f := range filters {
		f(eqCondition)
	}

	statement, args, err := query.Where(eqCondition).PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}
	count := make([]int, 0)
	if err = s.db.Select(&count, statement, args...); err != nil {
		return 0, err
	}
	return count[0], nil
}

func (s *store) SelectLogEntries(limit, offset uint64, filters ...practice.SQLFilter) ([]*practice.LogEntry, error) {
	eqCondition := make(squirrel.Eq)
	for _, f := range filters {
		f(eqCondition)
	}

	query := squirrel.Select("*").
		From(PracticeLogEntryTable).
		Limit(limit).
		Offset(offset).
		Where(eqCondition).
		OrderBy("date DESC")

	if val, hasKey := eqCondition["label_id"]; hasKey {
		if labelIDs, ok := val.([]string); ok && len(labelIDs) > 0 {
			query = squirrel.Select("id", "user_id", "date", "duration", "message").
				From(PracticeLogEntryTable).
				LeftJoin(fmt.Sprintf("%s ON %s.id = entry_id", AssociationPracticeLogEntryLabelTable, PracticeLogEntryTable)).
				Limit(limit).
				Offset(offset).
				Where(eqCondition).
				GroupBy(fmt.Sprintf("%s.id", PracticeLogEntryTable)).
				OrderBy("date DESC")
		}
	}

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows := make([]*DBPracticeLogEntry, 0)
	if err = s.db.Select(&rows, statement, args...); err != nil {
		return nil, err
	}

	entries := make([]*practice.LogEntry, 0)
	for _, row := range rows {
		entries = append(entries, row.toModel())
	}

	entryIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryIDs = append(entryIDs, entry.ID.String())
	}

	// This join can become expensive eventually.
	// But I want to have data consistency for labels.
	query = squirrel.
		Select("entry_id", "label_id", "parent_id", "name").
		From(PracticeLogLabelTable).
		LeftJoin(fmt.Sprintf("%s ON id = label_id", AssociationPracticeLogEntryLabelTable)).
		Where(squirrel.Eq{
			"entry_id": entryIDs,
		})

	statement, args, err = query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	labelRows := make([]*DBReadOnlyPracticeLogLabel, 0)
	if err = s.db.Select(&labelRows, statement, args...); err != nil {
		return nil, err
	}

	entryToLabels := make(map[uuid.UUID]map[uuid.UUID]*practice.LogLabel)
	for _, lbl := range labelRows {
		if _, ok := entryToLabels[lbl.EntryID]; !ok {
			entryToLabels[lbl.EntryID] = make(map[uuid.UUID]*practice.LogLabel)
		}
		entryToLabels[lbl.EntryID][lbl.ID] = &practice.LogLabel{
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

		entry.Labels = make([]*practice.LogLabel, 0, len(labels))
		for _, label := range labels {
			entry.Labels = append(entry.Labels, label)
		}
	}

	return entries, nil
}

func (s *store) CountLogEntriesByLabelIDs(labelIDs []string) {
	panic("implement me")
}

func (s *store) SelectLogEntriesByLabelIDs(limit, offset uint64, labelIDs []string) ([]*practice.LogEntry, error) {
	if len(labelIDs) == 0 {
		return nil, errors.New("please provide at least one label ID for this expensive query")
	}

	query := squirrel.Select("id", "user_id", "date", "duration", "message").
		From(PracticeLogEntryTable).
		LeftJoin(fmt.Sprintf("%s ON %s.id = entry_id", AssociationPracticeLogEntryLabelTable, PracticeLogEntryTable)).
		Limit(limit).
		Offset(offset).
		Where(squirrel.Eq{
			"label_id": labelIDs,
		}).
		GroupBy(fmt.Sprintf("%s.id", PracticeLogEntryTable)).
		OrderBy("date DESC")

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows := make([]*DBPracticeLogEntry, 0)
	if err = s.db.Select(&rows, statement, args...); err != nil {
		return nil, err
	}

	entries := make([]*practice.LogEntry, 0)
	for _, row := range rows {
		entries = append(entries, row.toModel())
	}

	return entries, nil
}

func (s *store) SelectLogLabels() ([]*practice.LogLabel, error) {
	query := squirrel.Select("*").From(PracticeLogLabelTable)

	statement, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows := make([]*DBPracticeLogLabel, 0)
	if err = s.db.Select(&rows, statement, args...); err != nil {
		return nil, err
	}

	labels := make([]*practice.LogLabel, 0)
	for _, row := range rows {
		labels = append(labels, row.toModel())
	}

	return labels, nil
}

func (s *store) UpdateLogEntry(entry *practice.LogEntry) error {
	newRow := new(DBPracticeLogEntry).fromModel(entry)

	updateQ := squirrel.Update(PracticeLogEntryTable).
		Set("user_id", newRow.UserID).
		Set("date", newRow.Date).
		Set("duration", newRow.Duration).
		Set("message", newRow.Message).
		Set("details", newRow.Details).
		Set("assignments", newRow.Assignments).
		Where(squirrel.Eq{"id": newRow.ID.String()})

	statement, args, err := updateQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return err
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			panic(err)
		}
	}()

	res, err := tx.Exec(statement, args...)
	if err != nil {
		return err
	}

	// Update association by removing everything and re-insert.
	deleteQ := squirrel.Delete(AssociationPracticeLogEntryLabelTable).
		Where(squirrel.Eq{"entry_id": newRow.ID.String()})

	statement, args, err = deleteQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(statement, args...)
	if err != nil {
		return err
	}

	// Re-insert associations
	joinQ := squirrel.Insert(AssociationPracticeLogEntryLabelTable).
		Columns("association_id", "entry_id", "label_id")
	for _, label := range entry.Labels {
		joinQ = joinQ.Values(uuid.New(), entry.ID, label.ID)
	}

	statement, args, err = joinQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(statement, args...)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction %w", err)
	}

	if count, err := res.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return errors.New("no row was affected, query did not work as intended")
	}
	return nil
}

func (s *store) UpdateLogAssignments(entry *practice.LogEntry) error {
	newRow := new(DBPracticeLogEntry).fromModel(entry)

	updateQ := squirrel.Update(PracticeLogEntryTable).
		Set("assignments", newRow.Assignments).
		Where(squirrel.Eq{"id": newRow.ID.String()})

	statement, args, err := updateQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return err
	}

	res, err := s.db.Exec(statement, args...)
	if err != nil {
		return err
	}

	if count, err := res.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return errors.New("no row was affected, query did not work as intended")
	}
	return nil
}

func (s *store) BatchInsertLogLabels(labels ...*practice.LogLabel) (int64, error) {
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

func (s *store) BatchInsertLogEntries(entries ...*practice.LogEntry) (int64, error) {
	entryInsertQ := squirrel.Insert(PracticeLogEntryTable).
		Columns("id", "user_id", "date", "duration", "message", "details", "assignments")

	joinInsertQ := squirrel.Insert(AssociationPracticeLogEntryLabelTable).
		Columns("association_id", "entry_id", "label_id")

	for _, entry := range entries {
		entry.ID = uuid.New()
		row := new(DBPracticeLogEntry).fromModel(entry)
		entryInsertQ = entryInsertQ.Values(
			row.ID, row.UserID, row.Date, row.Duration, row.Message, row.Details, row.Assignments)

		for _, label := range entry.Labels {
			joinInsertQ = joinInsertQ.Values(uuid.New(), entry.ID, label.ID)
		}
	}

	statement, args, err := entryInsertQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return 0, errors.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			panic(err)
		}
	}()

	res, err := tx.Exec(statement, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query %w", err)
	}

	statement, args, err = joinInsertQ.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to construct query")
	}

	_, err = tx.Exec(statement, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction %w", err)
	}

	return res.RowsAffected()
}
