package tinkoff

import (
	"context"
	"encoding/json"
	"time"

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
