package tinkoff

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/go-querystring/query"

	"github.com/jfk9w-go/based"
	"github.com/pkg/errors"
)

type investError struct {
	ErrorMessage string `json:"errorMessage"`
	ErrorCode    string `json:"errorCode"`
}

func (e investError) Error() string {
	return e.ErrorMessage + " (" + e.ErrorCode + ")"
}

type investExchange[R any] interface {
	path() string
	out() R
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

	urlQuery.Set("sessionId", sessionID)

	httpReq, err := http.NewRequest(http.MethodGet, baseURL+in.path(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "create http request")
	}

	httpReq.URL.RawQuery = urlQuery.Encode()
	httpReq.Header.Set("X-App-Name", "invest")
	httpReq.Header.Set("X-App-Version", "1.328.0")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "execute request")
	}

	if httpResp.Body == nil {
		return nil, errors.New(httpResp.Status)
	}

	defer httpResp.Body.Close()

	switch {
	case httpResp.StatusCode == http.StatusOK:
		var resp R
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return nil, errors.Wrap(err, "unmarshal response body")
		}

		return &resp, nil

	case httpResp.StatusCode >= 400 && httpResp.StatusCode < 600:
		var investErr investError
		if body, err := io.ReadAll(httpResp.Body); err != nil {
			return nil, errors.New(httpResp.Status)
		} else if err := json.Unmarshal(body, &investErr); err != nil {
			return nil, errors.New(ellipsis(body))
		} else {
			if investErr.ErrorCode == "404" {
				// this may be due to expired sessionid, try to check it
				if err := c.ping(ctx); errors.Is(err, errUnauthorized) {
					retry := &retryStrategy{
						timeout:    constantRetryTimeout(0),
						maxRetries: 1,
					}

					ctx, err := retry.do(ctx)
					if err != nil {
						return nil, investErr
					}

					if _, err := c.authorize(ctx); err != nil {
						return nil, errors.Wrap(err, "authorize")
					}

					return executeInvest[R](ctx, c, in)
				}
			}

			return nil, investErr
		}

	default:
		_, _ = io.Copy(io.Discard, httpResp.Body)
		return nil, errors.New(httpResp.Status)
	}
}

func ellipsis(data []byte) string {
	str := string(data)
	if len(str) > 200 {
		return str + "..."
	}

	return str
}

type DateTimeMilliOffset time.Time

func (dt DateTimeMilliOffset) Time() time.Time {
	return time.Time(dt)
}

func (dt *DateTimeMilliOffset) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	value, err := time.Parse("2006-01-02T15:04:05.999-07:00", str)
	if err != nil {
		return err
	}

	*dt = DateTimeMilliOffset(value)
	return nil
}

var dateLocation = &based.Lazy[*time.Location]{
	Fn: func(ctx context.Context) (*time.Location, error) {
		return time.LoadLocation("Europe/Moscow")
	},
}

type Date time.Time

func (d Date) Time() time.Time {
	return time.Time(d)
}

func (d *Date) UnmarshalJSON(data []byte) error {
	location, err := dateLocation.Get(context.Background())
	if err != nil {
		return errors.Wrap(err, "load location")
	}

	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	value, err := time.ParseInLocation("2006-01-02", str, location)
	if err != nil {
		return err
	}

	*d = Date(value)
	return nil
}

type investOperationTypesIn struct{}

func (in investOperationTypesIn) path() string {
	return "/invest-gw/ca-operations/api/v1/operations/types"
}

func (in investOperationTypesIn) out() (_ InvestOperationTypesOut) { return }

type InvestOperationType struct {
	Category      string `json:"category"`
	OperationName string `json:"operationName"`
	OperationType string `json:"operationType"`
}

type InvestOperationTypesOut struct {
	OperationsTypes []InvestOperationType `json:"operationsTypes"`
}

type InvestAmount struct {
	Currency string  `json:"currency"`
	Value    float64 `json:"value"`
}

type InvestAccountsIn struct {
	Currency string `url:"currency" validate:"required"`
}

func (in InvestAccountsIn) path() string               { return "/invest-gw/invest-portfolio/portfolios/accounts" }
func (in InvestAccountsIn) out() (_ InvestAccountsOut) { return }

type InvestTotals struct {
	ExpectedAverageYield         InvestAmount `json:"expectedAverageYield"`
	ExpectedAverageYieldRelative float64      `json:"expectedAverageYieldRelative"`
	ExpectedYield                InvestAmount `json:"expectedYield"`
	ExpectedYieldPerDay          InvestAmount `json:"expectedYieldPerDay"`
	ExpectedYieldPerDayRelative  float64      `json:"expectedYieldPerDayRelative"`
	ExpectedYieldRelative        float64      `json:"expectedYieldRelative"`
	TotalAmount                  InvestAmount `json:"totalAmount"`
}

type InvestAccount struct {
	AutoApp           bool   `json:"autoApp"`
	BrokerAccountId   string `json:"brokerAccountId"`
	BrokerAccountType string `json:"brokerAccountType"`
	BuyByDefault      bool   `json:"buyByDefault"`
	IsVisible         bool   `json:"isVisible"`
	Name              string `json:"name"`
	OpenedDate        Date   `json:"openedDate"`
	Order             int    `json:"order"`
	Organization      string `json:"organization"`
	Status            string `json:"status"`

	InvestTotals
}

type InvestAccounts struct {
	Count int             `json:"count"`
	List  []InvestAccount `json:"list"`
}

type InvestAccountsOut struct {
	Accounts InvestAccounts `json:"accounts"`
	Totals   InvestTotals   `json:"totals"`
}

type InvestOperationsIn struct {
	From               time.Time `url:"from,omitempty" layout:"2006-01-02T15:04:05.999Z"`
	To                 time.Time `url:"to,omitempty" layout:"2006-01-02T15:04:05.999Z"`
	BrokerAccountId    string    `url:"brokerAccountId,omitempty"`
	OvernightsDisabled *bool     `url:"overnightsDisabled,omitempty"`
	Limit              int       `url:"limit,omitempty"`
	Cursor             string    `url:"cursor,omitempty"`
}

func (in InvestOperationsIn) path() string                 { return "/invest-gw/ca-operations/api/v1/user/operations" }
func (in InvestOperationsIn) out() (_ InvestOperationsOut) { return }

type Trade struct {
	Date     DateTimeMilliOffset `json:"date"`
	Num      string              `json:"num"`
	Price    InvestAmount        `json:"price"`
	Quantity int                 `json:"quantity"`
}

type TradesInfo struct {
	Trades     []Trade `json:"trades"`
	TradesSize int     `json:"tradesSize"`
}

type InvestOperation struct {
	AccountName                   string              `json:"accountName"`
	AssetUid                      string              `json:"assetUid"`
	BestExecuted                  bool                `json:"bestExecuted"`
	BrokerAccountId               string              `json:"brokerAccountId"`
	ClassCode                     string              `json:"classCode"`
	Cursor                        string              `json:"cursor"`
	Date                          DateTimeMilliOffset `json:"date"`
	Description                   string              `json:"description"`
	DoneRest                      int                 `json:"doneRest"`
	Id                            string              `json:"id"`
	InstrumentType                string              `json:"instrumentType"`
	InstrumentUid                 string              `json:"instrumentUid"`
	InternalId                    string              `json:"internalId"`
	IsBlockedTradeClearingAccount bool                `json:"isBlockedTradeClearingAccount"`
	Isin                          string              `json:"isin"`
	Name                          string              `json:"name"`
	Payment                       InvestAmount        `json:"payment"`
	PaymentEur                    InvestAmount        `json:"paymentEur"`
	PaymentRub                    InvestAmount        `json:"paymentRub"`
	PaymentUsd                    InvestAmount        `json:"paymentUsd"`
	PositionUid                   string              `json:"positionUid"`
	Price                         *InvestAmount       `json:"price"`
	Quantity                      int                 `json:"quantity"`
	ShortDescription              string              `json:"shortDescription"`
	ShowName                      string              `json:"showName"`
	Status                        string              `json:"status"`
	Ticker                        string              `json:"ticker"`
	TradesInfo                    *TradesInfo         `json:"tradesInfo"`
	Type                          string              `json:"type"`
	YieldRelative                 float64             `json:"yieldRelative"`
}

type InvestOperationsOut struct {
	HasNext    bool              `json:"hasNext"`
	Items      []InvestOperation `json:"items"`
	NextCursor string            `json:"nextCursor"`
}
