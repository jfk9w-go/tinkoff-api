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
	auth() bool
	path() string
	out() R
}

type DateTimeMilliOffset time.Time

func (dt DateTimeMilliOffset) Time() time.Time {
	return time.Time(dt)
}

const dateTimeMilliOffsetLayout = "2006-01-02T15:04:05.999-07:00"

func (dt DateTimeMilliOffset) MarshalJSON() ([]byte, error) {
	value := dt.Time().Format(dateTimeMilliOffsetLayout)
	return json.Marshal(value)
}

func (dt *DateTimeMilliOffset) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	value, err := time.Parse(dateTimeMilliOffsetLayout, str)
	if err != nil {
		return err
	}

	*dt = DateTimeMilliOffset(value)
	return nil
}

type DateTime time.Time

func (dt DateTime) Time() time.Time {
	return time.Time(dt)
}

const dateTimeLayout = "2006-01-02T15:04:05Z"

func (dt DateTime) MarshalJSON() ([]byte, error) {
	value := dt.Time().Format(dateTimeLayout)
	return json.Marshal(value)
}

func (dt *DateTime) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	value, err := time.Parse(dateTimeLayout, str)
	if err != nil {
		return err
	}

	*dt = DateTime(value)
	return nil
}

var dateLocation = based.LazyFuncRef[*time.Location](
	func(ctx context.Context) (*time.Location, error) {
		return time.LoadLocation("Europe/Moscow")
	},
)

type Date time.Time

func (d Date) Time() time.Time {
	return time.Time(d)
}

const dateLayout = "2006-01-02"

func (d Date) MarshalJSON() ([]byte, error) {
	location, err := dateLocation.Get(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "load location")
	}

	value := d.Time().In(location).Format(dateLayout)
	return json.Marshal(value)
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

type InvestCandleDate time.Time

func (d InvestCandleDate) Time() time.Time {
	return time.Time(d)
}

func (d InvestCandleDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Time().Unix())
}

func (d *InvestCandleDate) UnmarshalJSON(data []byte) error {
	var secs int64
	if err := json.Unmarshal(data, &secs); err != nil {
		return err
	}

	value := time.Unix(secs, 0)
	*d = InvestCandleDate(value)
	return nil
}

type investOperationTypesIn struct{}

func (in investOperationTypesIn) auth() bool { return false }

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

func (in InvestAccountsIn) auth() bool                 { return true }
func (in InvestAccountsIn) path() string               { return "/invest-gw/invest-portfolio/portfolios/accounts" }
func (in InvestAccountsIn) out() (_ InvestAccountsOut) { return }

type InvestTotals struct {
	ExpectedYield                InvestAmount `json:"expectedYield"`
	ExpectedYieldRelative        float64      `json:"expectedYieldRelative"`
	ExpectedYieldPerDay          InvestAmount `json:"expectedYieldPerDay"`
	ExpectedYieldPerDayRelative  float64      `json:"expectedYieldPerDayRelative"`
	ExpectedAverageYield         InvestAmount `json:"expectedAverageYield"`
	ExpectedAverageYieldRelative float64      `json:"expectedAverageYieldRelative"`
	TotalAmount                  InvestAmount `json:"totalAmount"`
}

type InvestAccount struct {
	BrokerAccountId   string `json:"brokerAccountId"`
	BrokerAccountType string `json:"brokerAccountType"`
	Name              string `json:"name"`
	OpenedDate        Date   `json:"openedDate"`
	Order             int    `json:"order"`
	Status            string `json:"status"`
	IsVisible         bool   `json:"isVisible"`
	Organization      string `json:"organization"`
	BuyByDefault      bool   `json:"buyByDefault"`
	MarginEnabled     bool   `json:"marginEnabled"`
	AutoApp           bool   `json:"autoApp"`

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

func (in InvestOperationsIn) auth() bool                   { return true }
func (in InvestOperationsIn) path() string                 { return "/invest-gw/ca-operations/api/v1/user/operations" }
func (in InvestOperationsIn) out() (_ InvestOperationsOut) { return }

type Trade struct {
	Date          DateTimeMilliOffset `json:"date"`
	Num           string              `json:"num"`
	Price         InvestAmount        `json:"price"`
	Quantity      int                 `json:"quantity"`
	Yield         *InvestAmount       `json:"yield,omitempty"`
	YieldRelative *float64            `json:"yieldRelative,omitempty"`
}

type TradesInfo struct {
	Trades     []Trade `json:"trades"`
	TradesSize int     `json:"tradesSize"`
}

type InvestChildOperation struct {
	Currency       string       `json:"currency"`
	Id             string       `json:"id"`
	InstrumentType string       `json:"instrumentType"`
	InstrumentUid  string       `json:"instrumentUid"`
	LogoName       string       `json:"logoName"`
	Payment        InvestAmount `json:"payment"`
	ShowName       string       `json:"showName"`
	Ticker         string       `json:"ticker"`
	Type           string       `json:"type"`
	Value          float64      `json:"value"`
}

type InvestOperation struct {
	AccountName                   string                 `json:"accountName"`
	AssetUid                      *string                `json:"assetUid,omitempty"`
	BestExecuted                  bool                   `json:"bestExecuted"`
	BrokerAccountId               string                 `json:"brokerAccountId"`
	ClassCode                     *string                `json:"classCode,omitempty"`
	Cursor                        string                 `json:"cursor"`
	Date                          DateTimeMilliOffset    `json:"date"`
	Description                   string                 `json:"description"`
	Id                            *string                `json:"id,omitempty"`
	InstrumentType                *string                `json:"instrumentType,omitempty"`
	InstrumentUid                 *string                `json:"instrumentUid,omitempty"`
	InternalId                    string                 `json:"internalId"`
	IsBlockedTradeClearingAccount *bool                  `json:"isBlockedTradeClearingAccount,omitempty"`
	Isin                          *string                `json:"isin,omitempty"`
	LogoName                      *string                `json:"logoName,omitempty"`
	Name                          *string                `json:"name,omitempty"`
	Payment                       InvestAmount           `json:"payment"`
	PaymentEur                    InvestAmount           `json:"paymentEur"`
	PaymentRub                    InvestAmount           `json:"paymentRub"`
	PaymentUsd                    InvestAmount           `json:"paymentUsd"`
	PositionUid                   *string                `json:"positionUid,omitempty"`
	ShortDescription              *string                `json:"shortDescription,omitempty"`
	ShowName                      *string                `json:"showName,omitempty"`
	Status                        string                 `json:"status"`
	TextColor                     *string                `json:"textColor,omitempty"`
	Ticker                        *string                `json:"ticker,omitempty"`
	Type                          string                 `json:"type"`
	AccountId                     *string                `json:"accountId,omitempty"`
	DoneRest                      *int                   `json:"doneRest,omitempty"`
	Price                         *InvestAmount          `json:"price,omitempty"`
	Quantity                      *int                   `json:"quantity,omitempty"`
	TradesInfo                    *TradesInfo            `json:"tradesInfo,omitempty"`
	ParentOperationId             *string                `json:"parentOperationId,omitempty"`
	ChildOperations               []InvestChildOperation `json:"childOperations,omitempty"`
	Commission                    *InvestAmount          `json:"commission,omitempty"`
	Yield                         *InvestAmount          `json:"yield,omitempty"`
	YieldRelative                 *float64               `json:"yieldRelative,omitempty"`
	CancelReason                  *string                `json:"cancelReason,omitempty"`
	QuantityRest                  *int                   `json:"quantityRest,omitempty"`
	WithdrawDateTime              *DateTime              `json:"withdrawDateTime,omitempty"`
}

type InvestOperationsOut struct {
	HasNext    bool              `json:"hasNext"`
	Items      []InvestOperation `json:"items"`
	NextCursor string            `json:"nextCursor"`
}

type InvestCandlesIn struct {
	From       time.Time `url:"from" layout:"2006-01-02T15:04:05+00:00" validate:"required"`
	To         time.Time `url:"to" layout:"2006-01-02T15:04:05+00:00" validate:"required"`
	Resolution any       `url:"resolution" validate:"required"`
	Ticker     string    `url:"ticker" validate:"required"`
}

func (InvestCandlesIn) auth() auth                { return force }
func (InvestCandlesIn) path() string              { return "/api/trading/symbols/candles" }
func (InvestCandlesIn) out() (_ InvestCandlesOut) { return }
func (InvestCandlesIn) exprc() string             { return "OK" }

type InvestCandle struct {
	O    float64          `json:"o"`
	C    float64          `json:"c"`
	H    float64          `json:"h"`
	L    float64          `json:"l"`
	V    float64          `json:"v"`
	Date InvestCandleDate `json:"date"`
}

type InvestCandlesOut struct {
	Candles []InvestCandle `json:"candles"`
}
