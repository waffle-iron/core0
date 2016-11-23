package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

type ipfsStor struct {
	url    string
	client *http.Client
}

func NewIPFSStorage(u *url.URL) (Storage, error) {
	us := url.URL{
		Scheme: "http",
		Host:   u.Host,
		Path:   path.Join(u.Path, "api", "v0"),
	}

	return &ipfsStor{
		url:    us.String(),
		client: &http.Client{},
	}, nil
}

func (s *ipfsStor) Get(hash string) (io.ReadCloser, error) {
	request, err := http.NewRequest("POST",
		fmt.Sprintf("%s/cat/%s", s.url, hash), nil,
	)

	if err != nil {
		return nil, err
	}

	/* NOTE:
	   We do the following trick to be able to timeout the request ONLY if
	   no response is coming back from ipfs server. But when the data is
	   starting to flow back, we should not timeout even if downloading the
	   file is taking long time (if the file is really big)

	   Problem with setting client.Timeout will cancel the request even if
	   the file is being downloaded but taking longer time that the timeout
	*/

	ctx, cancel := context.WithCancel(context.Background())
	request = request.WithContext(ctx)
	var response *http.Response
	wait := make(chan struct{})
	defer close(wait)

	go func() {
		response, err = s.client.Do(request)
		wait <- struct{}{}
	}()

loop:
	for {
		select {
		case <-wait:
			break loop
		case <-time.After(15 * time.Second):
			cancel()
		}
	}

	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response error: %s", response.Status)
	}

	return response.Body, nil
}
