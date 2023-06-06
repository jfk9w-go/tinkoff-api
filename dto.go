package tinkoff

import (
	"encoding/json"
	"time"
)

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

type Currency struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	StrCode string `json:"strCode"`
}

type Amount struct {
	Currency Currency `json:"currency"`
	Value    float64  `json:"value"`
}

type AccountsLightIbIn struct{}

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
