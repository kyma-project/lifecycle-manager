package internal

import (
	"bufio"
	"io"
	"net/http"
	"os"
)

func download(filepath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	fileWriter := bufio.NewWriter(file)
	if _, err := fileWriter.ReadFrom(resp.Body); err != nil {
		return err
	}
	if err := fileWriter.Flush(); err != nil {
		return err
	}
	return nil
}
