package tinkoff

import (
	"context"
	"net/http"
	"time"

	"github.com/go-playground/validator"
	"github.com/jfk9w-go/based"
	"github.com/pkg/errors"
)

const baseURL = "https://www.tinkoff.ru/api"

var (
	ErrNoDataFound        = errors.New("no data found")
	errMaxRetriesExceeded = errors.New("max retries exceeded")
	errUnauthorized       = errors.New("no sessionid")
)

type Session struct {
	ID string
}

type SessionStorage interface {
	LoadSession(ctx context.Context, phone string) (*Session, error)
	UpdateSession(ctx context.Context, phone string, session *Session) error
}

type ConfirmationProvider interface {
	GetConfirmationCode(ctx context.Context, phone string) (string, error)
}

type Credential struct {
	Phone    string
	Password string
}

var validate = based.Lazy[*validator.Validate]{
	Fn: func(ctx context.Context) (*validator.Validate, error) {
		return validator.New(), nil
	},
}

type ClientBuilder struct {
	Clock                based.Clock          `validate:"required"`
	Credential           Credential           `validate:"required"`
	ConfirmationProvider ConfirmationProvider `validate:"required"`
	SessionStorage       SessionStorage       `validate:"required"`

	Transport http.RoundTripper
}

func (b ClientBuilder) Build(ctx context.Context) (*Client, error) {
	if validate, err := validate.Get(ctx); err != nil {
		return nil, err
	} else if err := validate.Struct(b); err != nil {
		return nil, err
	}

	c := &Client{
		credential: b.Credential,
		httpClient: &http.Client{
			Transport: b.Transport,
		},
		confirmationProvider: b.ConfirmationProvider,
		session: based.NewWriteThroughCached[string, *Session](
			based.WriteThroughCacheStorageFunc[string, *Session]{
				LoadFn:   b.SessionStorage.LoadSession,
				UpdateFn: b.SessionStorage.UpdateSession,
			},
			b.Credential.Phone,
		),
		rateLimiters: map[string]based.Locker{
			ShoppingReceiptIn{}.path(): based.Lockers{
				based.Semaphore(b.Clock, 25, 75*time.Second),
				based.Semaphore(b.Clock, 75, 11*time.Minute),
			},
		},
	}

	c.cancel = based.GoWithFeedback(context.Background(), context.WithCancel, func(ctx context.Context) {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			_ = c.ping(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})

	return c, nil
}

type Client struct {
	credential           Credential
	httpClient           *http.Client
	confirmationProvider ConfirmationProvider
	session              *based.WriteThroughCached[*Session]
	rateLimiters         map[string]based.Locker
	cancel               context.CancelFunc
	mu                   based.RWMutex
}

func (c *Client) AccountsLightIb(ctx context.Context) (AccountsLightIbOut, error) {
	resp, err := executeCommon[AccountsLightIbOut](ctx, c, accountsLightIbIn{})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

func (c *Client) Operations(ctx context.Context, in *OperationsIn) (OperationsOut, error) {
	resp, err := executeCommon[OperationsOut](ctx, c, in)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

func (c *Client) ShoppingReceipt(ctx context.Context, in *ShoppingReceiptIn) (*ShoppingReceiptOut, error) {
	resp, err := executeCommon[ShoppingReceiptOut](ctx, c, in)
	if err != nil {
		return nil, err
	}

	return &resp.Payload, nil
}

func (c *Client) InvestOperationTypes(ctx context.Context) (*InvestOperationTypesOut, error) {
	return executeInvest[InvestOperationTypesOut](ctx, c, investOperationTypesIn{})
}

func (c *Client) InvestAccounts(ctx context.Context, in *InvestAccountsIn) (*InvestAccountsOut, error) {
	return executeInvest[InvestAccountsOut](ctx, c, in)
}

func (c *Client) InvestOperations(ctx context.Context, in *InvestOperationsIn) (*InvestOperationsOut, error) {
	return executeInvest[InvestOperationsOut](ctx, c, in)
}

func (c *Client) Close() {
	c.cancel()
}

func (c *Client) rateLimiter(path string) based.Locker {
	if rateLimiter, ok := c.rateLimiters[path]; ok {
		return rateLimiter
	}

	return based.Unlock
}

func (c *Client) getSessionID(ctx context.Context) (string, error) {
	session, err := c.session.Get(ctx)
	if err != nil {
		return "", errors.Wrap(err, "get sessionid")
	}

	if session == nil {
		return "", errUnauthorized
	}

	return session.ID, nil
}

func (c *Client) ensureSessionID(ctx context.Context) (string, error) {
	session, err := c.session.Get(ctx)
	if err != nil {
		return "", err
	}

	if session == nil {
		if session, err = c.authorize(ctx); err != nil {
			_ = c.resetSessionID(ctx)
			return "", err
		}
	}

	return session.ID, nil
}

func (c *Client) resetSessionID(ctx context.Context) error {
	return c.session.Update(ctx, nil)
}

func (c *Client) authorize(ctx context.Context) (*Session, error) {
	var session *Session
	if resp, err := executeCommon[sessionOut](ctx, c, sessionIn{}); err != nil {
		return nil, errors.Wrap(err, "get new sessionid")
	} else {
		session = &Session{ID: resp.Payload}
		if err := c.session.Update(ctx, session); err != nil {
			return nil, errors.Wrap(err, "store new sessionid")
		}
	}

	if resp, err := executeCommon[signUpOut](ctx, c, phoneSignUpIn{Phone: c.credential.Phone}); err != nil {
		return nil, errors.Wrap(err, "phone sign up")
	} else {
		code, err := c.confirmationProvider.GetConfirmationCode(ctx, c.credential.Phone)
		if err != nil {
			return nil, errors.Wrap(err, "get confirmation code")
		}

		if _, err := executeCommon[confirmOut](ctx, c, confirmIn{
			InitialOperation:       "sign_up",
			InitialOperationTicket: resp.OperationTicket,
			ConfirmationData:       confirmationData{SMSBYID: code},
		}); err != nil {
			return nil, errors.Wrap(err, "submit confirmation code")
		}
	}

	if _, err := executeCommon[signUpOut](ctx, c, passwordSignUpIn{Password: c.credential.Password}); err != nil {
		return nil, errors.Wrap(err, "password sign up")
	}

	if _, err := executeCommon[levelUpOut](ctx, c, levelUpIn{}); err != nil {
		return nil, errors.Wrap(err, "level up")
	}

	return session, nil
}

func (c *Client) ping(ctx context.Context) error {
	ctx, cancel := c.mu.Lock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	out, err := executeCommon[pingOut](ctx, c, pingIn{})
	if err != nil {
		return errors.Wrap(err, "ping")
	}

	if out.Payload.AccessLevel != "CLIENT" {
		if err := c.resetSessionID(ctx); err != nil {
			return errors.Wrap(err, "reset sessionid")
		}

		return errUnauthorized
	}

	return nil
}
