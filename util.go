package dbot

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
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

func LogInput(auth string) {
	log(auth, color.FgMagenta)
}

func LogError(head string, body string) {
	log(head, color.FgYellow, getStandradOut(body), color.FgRed)
}

func LogNotice(head string, body string) {
	log(head, color.FgYellow, getStandradOut(body), color.FgBlue)
}

func LogCommandOut(head string, commnad string, out string) {
	log(
		head, color.FgYellow,
		getStandradOut(commnad), color.FgBlue,
		getStandradOut(out), color.FgGreen,
	)
}

func LogCommandErr(head string, commnad string, out string) {
	log(
		head, color.FgYellow,
		getStandradOut(commnad), color.FgBlue,
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
	LogInput(head)
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

func GetAbsConfigPathFrom(
	currentAbsConfig string,
	relativePath string,
) (string, error) {
	return filepath.Abs(
		filepath.Join(filepath.Dir(currentAbsConfig), relativePath),
	)
}

func ParseCommand(str string) []string {
	command := " " + str + " "
	ret := make([]string, 0)
	isSingleQuote := false
	isDoubleQuotes := false
	preChar := uint8(0)
	cmdStart := -1

	for i := 0; i < len(command); i++ {
		if isSingleQuote {
			if command[i] == 0x27 {
				isSingleQuote = false
			}
			preChar = command[i]
			continue
		}

		if isDoubleQuotes {
			if command[i] == 0x22 && preChar != 0x5C {
				isDoubleQuotes = false
			}
			preChar = command[i]
			continue
		}

		if command[i] == ' ' {
			if cmdStart >= 0 {
				ret = append(ret, command[cmdStart:i])
				cmdStart = -1
			}
			preChar = command[i]
			continue
		}

		if cmdStart < 0 {
			cmdStart = i
		}

		if command[i] == 0x27 {
			isSingleQuote = true
		}

		if command[i] == 0x22 {
			isDoubleQuotes = true
		}

		preChar = command[i]
	}

	return ret
}
