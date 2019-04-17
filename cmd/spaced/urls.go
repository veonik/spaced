package main

import (
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/veonik/eokvin"
)

type urlsKind int
const (
	urlsKindEokvin urlsKind = iota
)

type urlShortenerConfig struct {
	Kind urlsKind
	Options map[string]string `toml:"options"`
}

type urlShortener interface {
	NewShortURL(u *url.URL, shareTTL time.Duration) (*url.URL, error)
}

type eokvinShortener struct {
	*eokvin.Client
}

func (c *eokvinShortener) NewShortURL(u *url.URL, shareTTL time.Duration) (*url.URL, error) {
	su, err := c.Client.NewShortURL(u, shareTTL)
	if err != nil {
		return nil, err
	}
	return &su.URL, nil
}

func (c urlShortenerConfig) GetService() (urlShortener, error) {
	switch c.Kind {
	case urlsKindEokvin:
		token := c.Options["token"]
		endpoint := c.Options["endpoint"]
		if token == "" || endpoint == "" {
			return nil, errors.New("options 'token' and 'endpoint' are required")
		}
		c := eokvin.NewClient(endpoint, token)
		return &eokvinShortener{Client: c}, nil
	default:
		return nil, errors.Errorf("unsupported URL shortener kind %s", c.Kind)
	}
}
