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

func MergeEnv(
	parentEvn map[string]string, env map[string]string,
) map[string]string {
	ret := make(map[string]string)
	for key, value := range parentEvn {
		ret[key] = value
	}

	for key, value := range env {
		ret[key] = value
	}

	return ret
}

func ReplaceStringByEnv(str string, env map[string]string) string {
	replaceArray := make([]string, 0)
	for key, value := range env {
		replaceArray = append(replaceArray, "${"+key+"}", value)
	}
	replacer := strings.NewReplacer(replaceArray...)
	return replacer.Replace(str)
}

func ReadStringFromIOReader(reader io.Reader) (string, error) {
	var b bytes.Buffer

	if _, e := io.Copy(&b, reader); e != nil && e != io.EOF {
		return "", e
	}

	return b.String(), nil
}

func WriteStringToIOWriter(str string, writer io.Writer) (ret error) {
	if str == "" {
		return nil
	}

	strReader := strings.NewReader(str)
	if _, e := io.Copy(writer, strReader); e != nil && e != io.EOF {
		return e
	}

	return nil
}

func GetPasswordFromUser(head string) (string, error) {
	_, _ = color.New(color.FgMagenta, color.Bold).Print(head)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}
