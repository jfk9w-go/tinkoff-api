package tinkoff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/google/go-querystring/query"
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

	client := &Client{
		credential: b.Credential,
		httpClient: &http.Client{
			Transport: b.Transport,
		},
		confirmationProvider: b.ConfirmationProvider,
		session: based.NewWriteThroughCached[string, *Session](
			based.WriteThroughCacheStorageFunc[string, *Session]{
				LoadFn: func(ctx context.Context, key string) (*Session, error) {
					return b.SessionStorage.LoadSession(ctx, key)
				},
				UpdateFn: func(ctx context.Context, key string, value *Session) error {
					return b.SessionStorage.UpdateSession(ctx, key, value)
				},
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

	client.cancel = based.GoWithFeedback(context.Background(), context.WithCancel, client.ping)

	return client, nil
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
	resp, err := execute[AccountsLightIbOut](ctx, c, accountsLightIbIn{})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

func (c *Client) Operations(ctx context.Context, in *OperationsIn) (OperationsOut, error) {
	resp, err := execute[OperationsOut](ctx, c, in)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

func (c *Client) ShoppingReceipt(ctx context.Context, in *ShoppingReceiptIn) (*ShoppingReceiptOut, error) {
	resp, err := execute[ShoppingReceiptOut](ctx, c, in)
	if err != nil {
		return nil, err
	}

	return &resp.Payload, nil
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
	if resp, err := execute[sessionOut](ctx, c, sessionIn{}); err != nil {
		return nil, errors.Wrap(err, "get new sessionid")
	} else {
		session = &Session{ID: resp.Payload}
		if err := c.session.Update(ctx, session); err != nil {
			return nil, errors.Wrap(err, "store new sessionid")
		}
	}

	if resp, err := execute[signUpOut](ctx, c, phoneSignUpIn{Phone: c.credential.Phone}); err != nil {
		return nil, errors.Wrap(err, "phone sign up")
	} else {
		code, err := c.confirmationProvider.GetConfirmationCode(ctx, c.credential.Phone)
		if err != nil {
			return nil, errors.Wrap(err, "get confirmation code")
		}

		if _, err := execute[confirmOut](ctx, c, confirmIn{
			InitialOperation:       "sign_up",
			InitialOperationTicket: resp.OperationTicket,
			ConfirmationData:       confirmationData{SMSBYID: code},
		}); err != nil {
			return nil, errors.Wrap(err, "submit confirmation code")
		}
	}

	if _, err := execute[signUpOut](ctx, c, passwordSignUpIn{Password: c.credential.Password}); err != nil {
		return nil, errors.Wrap(err, "password sign up")
	}

	if _, err := execute[levelUpOut](ctx, c, levelUpIn{}); err != nil {
		return nil, errors.Wrap(err, "level up")
	}

	return session, nil
}

func (c *Client) ping(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if ctx, cancel := c.mu.Lock(ctx); ctx.Err() == nil {
			out, err := execute[pingOut](ctx, c, pingIn{})
			if err == nil && out.Payload.AccessLevel != "CLIENT" {
				_ = c.resetSessionID(ctx)
			}

			cancel()
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func execute[R any](ctx context.Context, c *Client, in exchange[R]) (*response[R], error) {
	var sessionID string
	if in.auth() != none {
		var (
			cancel context.CancelFunc
			err    error
		)

		ctx, cancel = c.mu.Lock(ctx)
		defer cancel()
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		switch in.auth() {
		case force:
			sessionID, err = c.ensureSessionID(ctx)
		case check:
			sessionID, err = c.getSessionID(ctx)
		default:
			return nil, errors.Errorf("unsupported auth %v", in.auth())
		}

		if err != nil {
			return nil, errors.Wrap(err, "get sessionid")
		}
	}

	ctx, cancel := c.rateLimiter(in.path()).Lock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	reqBody, err := query.Values(in)
	if err != nil {
		return nil, errors.Wrap(err, "encode form values")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+in.path(), strings.NewReader(reqBody.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	urlQuery := make(url.Values)
	urlQuery.Set("origin", "web,ib5,platform")
	if sessionID != "" {
		urlQuery.Set("sessionid", sessionID)
	}

	httpReq.URL.RawQuery = urlQuery.Encode()
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "execute request")
	}

	if httpResp.Body == nil {
		return nil, errors.New(httpResp.Status)
	}

	defer httpResp.Body.Close()

	var (
		respErr error
		retry   *retryStrategy
	)

	if httpResp.StatusCode != http.StatusOK {
		respErr = errors.New(httpResp.Status)
		retry = &retryStrategy{
			backOff:    exponentialRetryTimeout(time.Second, 2, 0.5),
			maxRetries: -1,
		}
	} else {
		var resp response[R]
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return nil, errors.Wrap(err, "decode response body")
		}

		if in.exprc() == resp.ResultCode {
			return &resp, nil
		}

		respErr = resultCodeError{
			actual:   resp.ResultCode,
			expected: in.exprc(),
			message:  resp.ErrorMessage,
		}

		switch resp.ResultCode {
		case "NO_DATA_FOUND":
			return nil, ErrNoDataFound

		case "REQUEST_RATE_LIMIT_EXCEEDED":
			retry = &retryStrategy{
				backOff:    exponentialRetryTimeout(time.Minute, 2, 0.2),
				maxRetries: 5,
			}

		case "INSUFFICIENT_PRIVILEGES":
			if _, err := c.authorize(ctx); err != nil {
				return nil, errors.Wrap(err, "authorize")
			}

			retry = &retryStrategy{
				backOff:    constantRetryTimeout(0),
				maxRetries: 1,
			}
		}
	}

	if retry != nil {
		ctx, retryErr := retry.do(ctx)
		switch {
		case errors.Is(retryErr, errMaxRetriesExceeded):
			// fallthrough
		case retryErr != nil:
			return nil, retryErr
		default:
			return execute[R](ctx, c, in)
		}
	}

	return nil, respErr
}

func executeInvest[R any](ctx context.Context, c *Client, in investExchange[R]) (*R, error) {
	ctx, cancel := c.mu.Lock(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sessionID, err := c.ensureSessionID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ensure sessionid")
	}

	urlQuery, err := query.Values(in)
	if err != nil {
		return nil, errors.Wrap(err, "encode url query")
	}

	urlQuery.Set("sessionid", sessionID)

	httpReq, err := http.NewRequest(http.MethodGet, baseURL+in.path(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "create http request")
	}

	httpReq.URL.RawQuery = urlQuery.Encode()

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "execute request")
	}

	if httpResp.Body == nil {
		return nil, errors.New(httpResp.Status)
	}

	defer httpResp.Body.Close()

	panic("not implemented")
}
