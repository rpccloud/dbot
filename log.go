package dbot

type logRecordLevel int

const (
	recordLevelInfo logRecordLevel = iota
	recordLevelError
)

type logRecord struct {
	level   logRecordLevel
	runAt   string
	jobName string
	body    string
}

func newLogRecordInfo(runAt string, jobName string, body string) *logRecord {
	return &logRecord{
		level:   recordLevelInfo,
		runAt:   runAt,
		jobName: jobName,
		body:    body,
	}
}

func newLogRecordError(runAt string, jobName string, body string) *logRecord {
	return &logRecord{
		level:   recordLevelError,
		runAt:   runAt,
		jobName: jobName,
		body:    body,
	}
}
