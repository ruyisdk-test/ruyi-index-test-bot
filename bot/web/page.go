package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func PageGet(url string) ([]byte, error) {
	// auto handle 302
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ruyi-index-test-bot/0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status: %s", resp.Status)
	}
	buf := make([]byte, resp.ContentLength)

	_, err = io.ReadFull(resp.Body, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return buf, nil
}

func TestUrl(url string) (int, error) {
	// auto handle 302
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "ruyi-index-test-bot/0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("http status: %s", resp.Status)
	}
	buf := make([]byte, 32*1024)
	// TODO: test file header

	_, err = io.ReadFull(resp.Body, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
		return resp.StatusCode, fmt.Errorf("download failed: %w", err)
	}

	return resp.StatusCode, nil
}
