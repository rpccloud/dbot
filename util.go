package dbot

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var (
	authColor        = color.New(color.FgMagenta, color.Bold)
	headInfoColor    = color.New(color.FgGreen, color.Bold)
	headErrorColor   = color.New(color.FgRed, color.Bold)
	headJobColor     = color.New(color.FgYellow, color.Bold)
	headCommandColor = color.New(color.FgGreen, color.Bold)
	bodyInfoColor    = color.New(color.FgBlue, color.Bold)
	bodyErrorColor   = color.New(color.FgRed, color.Bold)
	bodyJobColor     = color.New(color.FgYellow, color.Bold)
	bodyCommandColor = color.New(color.FgGreen, color.Bold)
)

type logRecordLevel int

const (
	recordLevelInfo logRecordLevel = iota
	recordLevelError
	recordLevelCommand
	recordLevelJob
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

func newLogRecordCommand(runAt string, jobName string, body string) *logRecord {
	return &logRecord{
		level:   recordLevelCommand,
		runAt:   runAt,
		jobName: jobName,
		body:    body,
	}
}

func newLogRecordJob(runAt string, jobName string, body string) *logRecord {
	return &logRecord{
		level:   recordLevelJob,
		runAt:   runAt,
		jobName: jobName,
		body:    body,
	}
}

func ReadStringFromIOReader(reader io.Reader) (string, error) {
	var b bytes.Buffer

	if _, e := io.Copy(&b, reader); e != nil && e != io.EOF {
		return "", e
	}

	return b.String(), nil
}

func GetPasswordFromUser(head string) (string, error) {
	_, _ = authColor.Print(head)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}

func FilterString(str string, filter []string) bool {
	for _, v := range filter {
		if strings.Contains(str, v) {
			return true
		}
	}

	return false
}
