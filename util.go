package dbot

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var gLogLock sync.Mutex

func getStandradOut(s string) string {
	if s != "" && s[len(s)-1] != '\n' {
		return s + "\n"
	} else {
		return s
	}
}

func log(a ...interface{}) {
	gLogLock.Lock()
	defer gLogLock.Unlock()

	for i := 0; i < len(a); i += 2 {
		if s := a[i].(string); s != "" {
			_, _ = color.New(a[i+1].(color.Attribute), color.Bold).Print(s)
		}

	}
}

func LogAuth(auth string) {
	log(auth, color.FgMagenta)
}

func LogError(head string, e error) {
	log(head, color.FgRed, getStandradOut(e.Error()), color.FgRed)
}

func LogNotice(head string, body string) {
	log(head, color.FgGreen, body, color.FgYellow)
}

func LogCommandOut(head string, commnad string, out string) {
	log(
		head, color.FgGreen,
		commnad, color.FgYellow,
		" => \n", color.FgGreen,
		getStandradOut(out), color.FgBlue,
	)
}

func LogCommandErr(head string, commnad string, out string) {
	log(
		head, color.FgRed,
		commnad, color.FgRed,
		" => \n", color.FgRed,
		getStandradOut(out), color.FgRed,
	)
}

func ReadStringFromIOReader(reader io.Reader) (string, error) {
	var b bytes.Buffer

	if _, e := io.Copy(&b, reader); e != nil && e != io.EOF {
		return "", e
	}

	return b.String(), nil
}

func GetPasswordFromUser(head string) (string, error) {
	LogAuth(head)
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
