package csrf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type CsrfService interface {
	Create(ctx context.Context, exp time.Duration) (token string, err error)
	Validate(ctx context.Context, r *http.Request, token string) error
}

type csrf struct {
	CookieName string
	cache      interface{}
}

var _ CsrfService = (*csrf)(nil)

func (c *csrf) Create(ctx context.Context, exp time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("cannot read random data: %s", err)
	}
	token := hex.EncodeToString(b)

	// TODO: persist key/value in cache

	return token, nil
}

func (c *csrf) Validate(ctx context.Context, r *http.Request, value string) error {
	return errors.New("not implemented")
}

const csrfCookie = "csrf"
