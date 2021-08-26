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

func ReadStringFromIOReader(reader io.Reader) (string, error) {
	var b bytes.Buffer

	if _, e := io.Copy(&b, reader); e != nil && e != io.EOF {
		return "", e
	}

	return b.String(), nil
}

func WriteStringToIOWriteCloser(str string, writer io.WriteCloser) (ret error) {
	defer func() {
		if e := writer.Close(); e != nil && e != io.EOF && ret == nil {
			ret = e
		}
	}()

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
