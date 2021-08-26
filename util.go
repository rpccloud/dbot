package dbot

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func ReadStringFromIOReader(reader io.Reader) (string, error) {
	var b bytes.Buffer
	for {
		if _, e := io.Copy(&b, reader); e != nil {
			if e == io.EOF {
				return b.String(), nil
			} else {
				return "", e
			}
		}
	}
}

func WriteStringToIOWriteCloser(str string, writer io.WriteCloser) (ret error) {
	defer func() {
		if e := writer.Close(); e != nil && ret == nil {
			ret = e
		}
	}()

	if str == "" {
		return nil
	}

	strReader := strings.NewReader(str)
	for {
		if _, e := io.Copy(writer, strReader); e != nil {
			if e == io.EOF {
				return nil
			} else {
				return e
			}
		}
	}
}

func GetPasswordFromUser(head string) (string, error) {
	fmt.Print(head)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}
