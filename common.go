package tinkoff

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"
)

type auth int

const (
	none auth = iota
	check
	force
)

type commonExchange[R any] interface {
	auth() auth
	path() string
	out() R
	exprc() string
}

type commonResponse[R any] struct {
	ResultCode      string `json:"resultCode"`
	ErrorMessage    string `json:"errorMessage"`
	Payload         R      `json:"payload"`
	OperationTicket string `json:"operationTicket"`
}

type resultCodeError struct {
	expected, actual string
	message          string
}

func (e resultCodeError) Error() string {
	var b strings.Builder
	b.WriteString(e.actual)
	b.WriteString(" != ")
	b.WriteString(e.expected)
	if e.message != "" {
		b.WriteString(" (")
		b.WriteString(e.message)
		b.WriteString(")")
	}

	return b.String()
}

func executeCommon[R any](ctx context.Context, c *Client, in commonExchange[R]) (*commonResponse[R], error) {
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
		if body, err := io.ReadAll(httpResp.Body); err != nil {
			respErr = errors.New(httpResp.Status)
		} else {
			respErr = errors.New(ellipsis(body))
		}

		retry = &retryStrategy{
			timeout:    exponentialRetryTimeout(time.Second, 2, 0.5),
			maxRetries: -1,
		}
	} else {
		var resp commonResponse[R]
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
				timeout:    exponentialRetryTimeout(time.Minute, 2, 0.2),
				maxRetries: 5,
			}

		case "INSUFFICIENT_PRIVILEGES":
			if _, err := c.authorize(ctx); err != nil {
				return nil, errors.Wrap(err, "authorize")
			}

			retry = &retryStrategy{
				timeout:    constantRetryTimeout(0),
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
			return executeCommon[R](ctx, c, in)
		}
	}

	return nil, respErr
}

type Milliseconds time.Time

func (ms Milliseconds) Time() time.Time {
	return time.Time(ms)
}

func (ms *Milliseconds) UnmarshalJSON(data []byte) error {
	var value struct {
		Milliseconds int64 `json:"milliseconds"`
	}

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*ms = Milliseconds(time.UnixMilli(value.Milliseconds))
	return nil
}

type Seconds time.Time

func (s Seconds) Time() time.Time {
	return time.Time(s)
}

func (s *Seconds) UnmarshalJSON(data []byte) error {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*s = Seconds(time.Unix(value, 0))
	return nil
}

type sessionIn struct{}

func (in sessionIn) auth() auth          { return none }
func (in sessionIn) path() string        { return "/common/v1/session" }
func (in sessionIn) out() (_ sessionOut) { return }
func (in sessionIn) exprc() string       { return "OK" }

type sessionOut = string

type pingIn struct{}

func (in pingIn) auth() auth       { return check }
func (in pingIn) path() string     { return "/common/v1/ping" }
func (in pingIn) out() (_ pingOut) { return }
func (in pingIn) exprc() string    { return "OK" }

type pingOut struct {
	AccessLevel string `json:"accessLevel"`
}

type signUpIn struct{}

func (in signUpIn) auth() auth         { return check }
func (in signUpIn) path() string       { return "/common/v1/sign_up" }
func (in signUpIn) out() (_ signUpOut) { return }

type signUpOut = json.RawMessage

type phoneSignUpIn struct {
	signUpIn
	Phone string `url:"phone"`
}

func (in phoneSignUpIn) exprc() string { return "WAITING_CONFIRMATION" }

type passwordSignUpIn struct {
	signUpIn
	Password string `url:"password"`
}

func (in passwordSignUpIn) exprc() string { return "OK" }

type confirmationData struct {
	SMSBYID string `json:"SMSBYID"`
}

func (cd confirmationData) EncodeValues(key string, v *url.Values) error {
	data, err := json.Marshal(cd)
	if err != nil {
		return err
	}

	v.Set(key, string(data))
	return nil
}

type confirmIn struct {
	InitialOperation       string           `url:"initialOperation"`
	InitialOperationTicket string           `url:"initialOperationTicket"`
	ConfirmationData       confirmationData `url:"confirmationData"`
}

func (in confirmIn) auth() auth          { return check }
func (in confirmIn) path() string        { return "/common/v1/confirm" }
func (in confirmIn) out() (_ confirmOut) { return }
func (in confirmIn) exprc() string       { return "OK" }

type confirmOut = json.RawMessage

type levelUpIn struct{}

func (in levelUpIn) auth() auth          { return check }
func (in levelUpIn) path() string        { return "/common/v1/level_up" }
func (in levelUpIn) out() (_ levelUpOut) { return }
func (in levelUpIn) exprc() string       { return "OK" }

type levelUpOut = json.RawMessage

type Currency struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	StrCode string `json:"strCode"`
}

type Amount struct {
	Currency Currency `json:"currency"`
	Value    float64  `json:"value"`
}

type accountsLightIbIn struct{}

func (in accountsLightIbIn) auth() auth                  { return force }
func (in accountsLightIbIn) path() string                { return "/common/v1/accounts_light_ib" }
func (in accountsLightIbIn) out() (_ AccountsLightIbOut) { return }
func (in accountsLightIbIn) exprc() string               { return "OK" }

type MultiCardCluster struct {
	Id string `json:"id"`
}

type Card struct {
	CreationDate     Milliseconds     `json:"creationDate"`
	Expiration       Milliseconds     `json:"expiration"`
	FrozenCard       bool             `json:"frozenCard"`
	HasWrongPins     bool             `json:"hasWrongPins"`
	Id               string           `json:"id"`
	IsEmbossed       string           `json:"isEmbossed"`
	IsPaymentDevice  bool             `json:"isPaymentDevice"`
	IsVirtual        string           `json:"isVirtual"`
	MultiCardCluster MultiCardCluster `json:"multiCardCluster"`
	Name             string           `json:"name"`
	PaymentSystem    string           `json:"paymentSystem"`
	PinSec           bool             `json:"pinSec"`
	Primary          bool             `json:"primary"`
	Status           string           `json:"status"`
	StatusCode       string           `json:"statusCode"`
	Ucid             string           `json:"ucid"`
	Value            string           `json:"value"`
}

type Loyalty struct {
	AccrualBonuses        float64 `json:"accrualBonuses"`
	AvailableBonuses      float64 `json:"availableBonuses"`
	CashbackProgram       bool    `json:"cashbackProgram"`
	CoreGroup             string  `json:"coreGroup"`
	LinkedBonuses         string  `json:"linkedBonuses"`
	LoyaltyPointsId       int64   `json:"loyaltyPointsId"`
	ProgramCode           string  `json:"programCode"`
	TotalAvailableBonuses float64 `json:"totalAvailableBonuses"`
}

type Account struct {
	AccountType           string       `json:"accountType"`
	Cards                 []Card       `json:"cards"`
	ClientUnverifiedFlag  string       `json:"clientUnverifiedFlag"`
	CreationDate          Milliseconds `json:"creationDate"`
	CreditLimit           *Amount      `json:"creditLimit"`
	Currency              Currency     `json:"currency"`
	CurrentMinimalPayment *Amount      `json:"currentMinimalPayment"`
	DebtAmount            *Amount      `json:"debtAmount"`
	DueDate               Milliseconds `json:"dueDate"`
	Hidden                bool         `json:"hidden"`
	Id                    string       `json:"id"`
	LastStatementDate     Milliseconds `json:"lastStatementDate"`
	Loyalty               *Loyalty     `json:"loyalty"`
	LoyaltyId             string       `json:"loyaltyId"`
	MoneyAmount           *Amount      `json:"moneyAmount"`
	Name                  string       `json:"name"`
	NextStatementDate     Milliseconds `json:"nextStatementDate"`
	PartNumber            string       `json:"partNumber"`
	PastDueDebt           *Amount      `json:"pastDueDebt"`
	SharedByMeFlag        bool         `json:"sharedByMeFlag"`
	Status                string       `json:"status"`
}

type AccountsLightIbOut []Account

type OperationsIn struct {
	Account                string    `url:"account" validate:"required"`
	Start                  time.Time `url:"start,unixmilli" validate:"required"`
	End                    time.Time `url:"end,unixmilli,omitempty"`
	OperationId            string    `url:"operationId,omitempty"`
	TrancheCreationAllowed *bool     `url:"trancheCreationAllowed,omitempty"`
	LoyaltyPaymentProgram  string    `url:"loyaltyPaymentProgram,omitempty"`
	LoyaltyPaymentStatus   string    `url:"loyaltyPaymentStatus,omitempty"`
}

func (in OperationsIn) auth() auth             { return force }
func (in OperationsIn) path() string           { return "/common/v1/operations" }
func (in OperationsIn) out() (_ OperationsOut) { return }
func (in OperationsIn) exprc() string          { return "OK" }

type Category struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type LoyaltyAmount struct {
	Loyalty             string  `json:"loyalty"`
	LoyaltyImagine      bool    `json:"loyaltyImagine"`
	LoyaltyPointsId     int64   `json:"loyaltyPointsId"`
	LoyaltyPointsName   string  `json:"loyaltyPointsName"`
	LoyaltyProgramId    string  `json:"loyaltyProgramId"`
	LoyaltySteps        int     `json:"loyaltySteps"`
	Name                string  `json:"name"`
	PartialCompensation bool    `json:"partialCompensation"`
	Value               float64 `json:"value"`
}

type LoyaltyBonus struct {
	Amount           LoyaltyAmount `json:"amount"`
	CompensationType string        `json:"compensationType"`
	Description      string        `json:"description"`
	LoyaltyType      string        `json:"loyaltyType"`
}

type Region struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type Merchant struct {
	Name   string  `json:"name"`
	Region *Region `json:"region"`
}

type SpendingCategory struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Operation struct {
	Account                string         `json:"account"`
	AccountAmount          Amount         `json:"accountAmount"`
	Amount                 Amount         `json:"amount"`
	AuthorizationId        string         `json:"authorizationId"`
	Card                   string         `json:"card"`
	CardNumber             string         `json:"cardNumber"`
	CardPresent            bool           `json:"cardPresent"`
	Cashback               float64        `json:"cashback"`
	CashbackAmount         Amount         `json:"cashbackAmount"`
	Category               Category       `json:"category"`
	Compensation           string         `json:"compensation"`
	DebitingTime           Milliseconds   `json:"debitingTime"`
	Description            string         `json:"description"`
	Group                  string         `json:"group"`
	HasShoppingReceipt     bool           `json:"hasShoppingReceipt"`
	HasStatement           bool           `json:"hasStatement"`
	Id                     string         `json:"id"`
	IdSourceType           string         `json:"idSourceType"`
	InstallmentStatus      string         `json:"installmentStatus"`
	IsDispute              bool           `json:"isDispute"`
	IsExternalCard         bool           `json:"isExternalCard"`
	IsHce                  bool           `json:"isHce"`
	IsInner                bool           `json:"isInner"`
	IsOffline              bool           `json:"isOffline"`
	IsSuspicious           bool           `json:"isSuspicious"`
	IsTemplatable          bool           `json:"isTemplatable"`
	Locations              []Location     `json:"locations"`
	LoyaltyBonus           []LoyaltyBonus `json:"loyaltyBonus"`
	Mcc                    int            `json:"mcc"`
	MccString              string         `json:"mccString"`
	Merchant               Merchant       `json:"merchant"`
	OperationTime          Milliseconds   `json:"operationTime"`
	OperationTransferred   bool           `json:"operationTransferred"`
	PointOfSaleId          int64          `json:"pointOfSaleId"`
	PosId                  string         `json:"posId"`
	Status                 string         `json:"status"`
	TrancheCreationAllowed bool           `json:"trancheCreationAllowed"`
	Type                   string         `json:"type"`
	TypeSerno              int64          `json:"typeSerno"`
	Ucid                   string         `json:"ucid"`
	VirtualPaymentType     int            `json:"virtualPaymentType"`
}

type OperationsOut = []Operation

type ShoppingReceiptIn struct {
	OperationId   string    `url:"operationId" validate:"required"`
	OperationTime time.Time `url:"operationTime,unixmilli,omitempty"`
	IdSourceType  string    `url:"idSourceType,omitempty"`
	Account       string    `url:"account,omitempty"`
}

func (in ShoppingReceiptIn) auth() auth                  { return force }
func (in ShoppingReceiptIn) path() string                { return "/common/v1/shopping_receipt" }
func (in ShoppingReceiptIn) out() (_ ShoppingReceiptOut) { return }
func (in ShoppingReceiptIn) exprc() string               { return "OK" }

type ReceiptItem struct {
	BrandId  int64   `json:"brand_id"`
	GoodId   int64   `json:"good_id"`
	Name     string  `json:"name"`
	Nds      int     `json:"nds"`
	NdsRate  int     `json:"ndsRate"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Sum      float64 `json:"sum"`
}

type Receipt struct {
	AppliedTaxationType     int           `json:"appliedTaxationType"`
	CashTotalSum            float64       `json:"cashTotalSum"`
	CreditSum               float64       `json:"creditSum"`
	DateTime                Seconds       `json:"dateTime"`
	EcashTotalSum           float64       `json:"ecashTotalSum"`
	FiscalDocumentNumber    int64         `json:"fiscalDocumentNumber"`
	FiscalDriveNumber       int64         `json:"fiscalDriveNumber"`
	FiscalDriveNumberString string        `json:"fiscalDriveNumberString"`
	FiscalSign              int64         `json:"fiscalSign"`
	Items                   []ReceiptItem `json:"items"`
	KktRegId                string        `json:"kktRegId"`
	OperationType           int           `json:"operationType"`
	Operator                string        `json:"operator"`
	PrepaidSum              float64       `json:"prepaidSum"`
	ProvisionSum            float64       `json:"provisionSum"`
	RequestNumber           int64         `json:"requestNumber"`
	RetailPlace             string        `json:"retailPlace"`
	RetailPlaceAddress      string        `json:"retailPlaceAddress"`
	ShiftNumber             int64         `json:"shiftNumber"`
	TaxationType            int           `json:"taxationType"`
	TotalSum                float64       `json:"totalSum"`
	User                    string        `json:"user"`
	UserInn                 string        `json:"userInn"`
}

type ShoppingReceiptOut struct {
	OperationDateTime Milliseconds `json:"operationDateTime"`
	OperationId       string       `json:"operationId"`
	Receipt           Receipt      `json:"receipt"`
}
