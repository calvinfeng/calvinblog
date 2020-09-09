package cmd

import (
	"errors"
	"fmt"
	"github.com/calvinfeng/playground/practice"
	"github.com/calvinfeng/playground/practice/logstore"
	"github.com/calvinfeng/playground/trelloapi"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Reset the database, apply migrationsV1 and then seed it.
func manageDBRunE(_ *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("provide an argument to manage database [reset, migrate, seed]")
	}

	switch args[0] {
	case "reset":
		m, err := migrate.New("file://./migrationsV2", databaseAddress())
		if err != nil {
			return err
		}
		if err := m.Drop(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to drop V2 migrations: %w", err)
		}
		if version, dirty, err := m.Version(); err != nil && err != migrate.ErrNilVersion {
			return err
		} else {
			logrus.Infof("successfully reset database to version %d, dirty=%v", version, dirty)
		}
		return nil
	case "migrate":
		m, err := migrate.New("file://./migrationsV2", databaseAddress())
		if err != nil {
			return err
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to apply V2 migrations: %w", err)
		}
		if version, dirty, err := m.Version(); err != nil {
			return err
		} else {
			logrus.Infof("successfully migrated database to version %d, dirty=%v", version, dirty)
		}
		return nil
	case "seed":
		return seedFromTrello()
	default:
		return fmt.Errorf("%s is not a recognized command", args[0])
	}
}

func seedFromTrello() error {
	boardID := "woq8deqm"
	api := trelloapi.New(trelloapi.Config{
		TrelloAPIKey:   os.Getenv("TRELLO_API_KEY"),
		TrelloAPIToken: os.Getenv("TRELLO_API_TOKEN"),
	})

	pg, err := sqlx.Open("postgres", databaseAddress())
	if err != nil {
		return err
	}

	store := logstore.New(pg)
	logLabels := []*practice.LogLabel{
		{Name: "Acoustic"},
		{Name: "Blues"},
		{Name: "Finger Mechanics"},
		{Name: "Jam Sessions"},
		{Name: "Music Lessons"},
		{Name: "Songs"},
		{Name: "Scales"},
	}
	if inserted, err := store.BatchInsertLogLabels(logLabels...); err != nil {
		logrus.WithError(err).Error("failed to batch insert log labels")
	} else {
		logrus.Infof("inserted %d log labels", inserted)
	}

	subLogLabels := []*practice.LogLabel{
		{Name: "Acoustic Rhythm", ParentID: logLabels[0].ID},
		{Name: "Spider Walk", ParentID: logLabels[2].ID},
		{Name: "Legato", ParentID: logLabels[2].ID},
		{Name: "Barre Chords", ParentID: logLabels[2].ID},
		{Name: "Chord Change", ParentID: logLabels[2].ID},
		{Name: "Now & Forever", ParentID: logLabels[5].ID},
		{Name: "Final Countdown", ParentID: logLabels[5].ID},
		{Name: "海阔天空", ParentID: logLabels[5].ID},
	}

	if inserted, err := store.BatchInsertLogLabels(subLogLabels...); err != nil {
		logrus.WithError(err).Error("failed to batch insert log labels")
	} else {
		logrus.Infof("inserted %d log labels", inserted)
	}

	logLabelsByName := make(map[string]*practice.LogLabel)
	logLabels, err = store.SelectLogLabels()
	if err != nil {
		return err
	}

	for _, logLabel := range logLabels {
		logrus.Infof("found log label %s", logLabel.Name)
		logLabelsByName[logLabel.Name] = logLabel
	}

	trelloLabels, err := api.TrelloLabelsByBoard(boardID)
	if err != nil {
		return err
	}

	trelloLabelsByID := make(map[string]trelloapi.TrelloLabel)
	for _, label := range trelloLabels {
		trelloLabelsByID[label.ID] = label
	}

	cards, err := api.TrelloCardsByBoard(boardID)
	if err != nil {
		return err
	}

	entries := make([]*practice.LogEntry, 0, len(cards))
	for _, card := range cards {
		if card.IsTemplate {
			continue
		}

		entry := new(practice.LogEntry)
		entry.Title = card.Name
		entry.Note = card.Description
		entry.Labels = make([]*practice.LogLabel, 0)
		entry.UserID = "calvin.j.feng@gmail.com"
		for _, labelID := range card.LabelIDs {
			label, ok := trelloLabelsByID[labelID]
			if !ok {
				logrus.Fatalf("label %d not found", labelID)
			}

			if duration, err := time.ParseDuration(label.Name); err == nil {
				entry.Duration += int32(duration.Minutes())
			} else {
				switch label.Name {
				case "Barre Chords":
					entry.Labels = append(entry.Labels, logLabelsByName["Finger Mechanics"], logLabelsByName["Barre Chords"])
				case "Chord Change":
					entry.Labels = append(entry.Labels, logLabelsByName["Finger Mechanics"], logLabelsByName["Chord Change"])
				case "Rhythm":
					entry.Labels = append(entry.Labels, logLabelsByName["Acoustic"], logLabelsByName["Acoustic Rhythm"])
				default:
					if label, ok := logLabelsByName[label.Name]; ok {
						entry.Labels = append(entry.Labels, label)
					} else {
						logrus.Errorf("found unrecognized label name from Trello", label.Name)
					}
				}
			}
		}

		if card.Due == "" {
			continue
		}

		date, err := time.Parse(time.RFC3339, card.Due)
		if err != nil {
			logrus.WithError(err).Warn("failed to parse due date of card %s %s", card.ID, card.Name)
			continue
		}

		entry.Date = date
		if len(entry.Labels) == 0 {
			logrus.Warnf("log entry %s %s has no labels", entry.Title, entry.Date)
		}

		entries = append(entries, entry)
	}

	if inserted, err := store.BatchInsertLogEntries(entries...); err != nil {
		return err
	} else {
		logrus.Infof("inserted %d entries", inserted)
	}

	count, err := store.CountLogEntries()
	if err != nil {
		return err
	}

	entries, err = store.SelectLogEntries(uint64(count), 0)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		logrus.Infof("entry %s %s has labels", entry.ID, entry.Title, entry.Labels)
	}

	return nil
}
