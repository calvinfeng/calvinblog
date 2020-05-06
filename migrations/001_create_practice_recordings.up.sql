CREATE TABLE practice_recordings (
    id INTEGER PRIMARY KEY,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    day INTEGER NOT NULL,
    is_progress_report INT2 NOT NULL,
    youtube_video_id VARCHAR(255) NOT NULL,
    video_orientation VARCHAR(255) NOT NULL,
    title TEXT
);

CREATE INDEX practice_recordings_year_index ON practice_recordings(year);
CREATE INDEX practice_recordings_month_index ON practice_recordings(month);
CREATE INDEX is_progress_report_index on practice_recordings(is_progress_report);