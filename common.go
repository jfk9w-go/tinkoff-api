package tinkoff

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

const (
	shoppingReceiptPath = "/common/v1/shopping_receipt"
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

type Milliseconds time.Time

func (ms Milliseconds) Time() time.Time {
	return time.Time(ms)
}

func (ms Milliseconds) MarshalJSON() ([]byte, error) {
	var value struct {
		Milliseconds int64 `json:"milliseconds"`
	}

	value.Milliseconds = ms.Time().UnixMilli()
	return json.Marshal(value)
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

func (s Seconds) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Time().Unix())
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
	Code    uint   `json:"code"`
	Name    string `json:"name"`
	StrCode string `json:"strCode"`
}

type MoneyAmount struct {
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
	Id               string            `json:"id"`
	StatusCode       string            `json:"statusCode"`
	Status           string            `json:"status"`
	PinSet           bool              `json:"pinSet"`
	Expiration       Milliseconds      `json:"expiration"`
	CardDesign       string            `json:"cardDesign"`
	Ucid             string            `json:"ucid"`
	PaymentSystem    string            `json:"paymentSystem"`
	FrozenCard       bool              `json:"frozenCard"`
	HasWrongPins     bool              `json:"hasWrongPins"`
	Value            string            `json:"value"`
	IsEmbossed       bool              `json:"isEmbossed"`
	IsVirtual        bool              `json:"isVirtual"`
	CreationDate     Milliseconds      `json:"creationDate"`
	MultiCardCluster *MultiCardCluster `json:"multiCardCluster,omitempty"`
	Name             string            `json:"name"`
	IsPaymentDevice  bool              `json:"isPaymentDevice"`
	Primary          bool              `json:"primary"`
	CardIssueType    string            `json:"cardIssueType"`
	SharedResourceId *string           `json:"sharedResourceId,omitempty"`
}

type Loyalty struct {
	ProgramName            string   `json:"programName"`
	ProgramCode            string   `json:"programCode"`
	AccountBackgroundColor string   `json:"accountBackgroundColor"`
	CashbackProgram        bool     `json:"cashbackProgram"`
	CoreGroup              string   `json:"coreGroup"`
	LoyaltyPointsId        uint8    `json:"loyaltyPointsId"`
	AccrualBonuses         *float64 `json:"accrualBonuses,omitempty"`
	LinkedBonuses          *string  `json:"linkedBonuses,omitempty"`
	TotalAvailableBonuses  *float64 `json:"totalAvailableBonuses,omitempty"`
	AvailableBonuses       *float64 `json:"availableBonuses,omitempty"`
}

type AccountShared struct {
	Scopes     []string     `json:"scopes"`
	StartDate  Milliseconds `json:"startDate"`
	OwnerName  string       `json:"ownerName"`
	SharStatus string       `json:"sharStatus"`
}

type Account struct {
	Id                    string            `json:"id"`
	Currency              *Currency         `json:"currency,omitempty"`
	CreditLimit           *MoneyAmount      `json:"creditLimit,omitempty"`
	MoneyAmount           *MoneyAmount      `json:"moneyAmount,omitempty"`
	DebtBalance           *MoneyAmount      `json:"debtBalance,omitempty"`
	CurrentMinimalPayment *MoneyAmount      `json:"currentMinimalPayment,omitempty"`
	ClientUnverifiedFlag  *string           `json:"clientUnverifiedFlag,omitempty"`
	IdentificationState   *string           `json:"identificationState,omitempty"`
	Status                *string           `json:"status,omitempty"`
	EmoneyFlag            *bool             `json:"emoneyFlag,omitempty"`
	NextStatementDate     *Milliseconds     `json:"nextStatementDate,omitempty"`
	DueDate               *Milliseconds     `json:"dueDate,omitempty"`
	Cards                 []Card            `json:"cards,omitempty"`
	MultiCardCluster      *MultiCardCluster `json:"multiCardCluster,omitempty"`
	LoyaltyId             *string           `json:"loyaltyId,omitempty"`
	MoneyPotFlag          *bool             `json:"moneyPotFlag,omitempty"`
	PartNumber            *string           `json:"partNumber,omitempty"`
	PastDueDebt           *MoneyAmount      `json:"pastDueDebt,omitempty"`
	Name                  string            `json:"name"`
	AccountType           string            `json:"accountType"`
	Hidden                bool              `json:"hidden"`
	SharedByMeFlag        *bool             `json:"sharedByMeFlag,omitempty"`
	Loyalty               *Loyalty          `json:"loyalty,omitempty"`
	CreationDate          *Milliseconds     `json:"creationDate,omitempty"`
	DebtAmount            *MoneyAmount      `json:"debtAmount,omitempty"`
	LastStatementDate     *Milliseconds     `json:"lastStatementDate,omitempty"`
	DueColor              *int              `json:"dueColor,omitempty"`
	LinkedAccountNumber   *string           `json:"linkedAccountNumber,omitempty"`
	IsKidsSaving          *bool             `json:"isKidsSaving,omitempty"`
	IsCrowdfunding        *bool             `json:"isCrowdfunding,omitempty"`
	Shared                *AccountShared    `json:"shared,omitempty"`
}

type AccountsLightIbOut = []Account

type OperationsIn struct {
	Account                string     `url:"account" validate:"required"`
	Start                  time.Time  `url:"start,unixmilli" validate:"required"`
	End                    *time.Time `url:"end,unixmilli,omitempty"`
	OperationId            *string    `url:"operationId,omitempty"`
	TrancheCreationAllowed *bool      `url:"trancheCreationAllowed,omitempty"`
	LoyaltyPaymentProgram  *string    `url:"loyaltyPaymentProgram,omitempty"`
	LoyaltyPaymentStatus   *string    `url:"loyaltyPaymentStatus,omitempty"`
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
	Value               float64 `json:"value"`
	LoyaltyProgramId    string  `json:"loyaltyProgramId"`
	Loyalty             string  `json:"loyalty"`
	Name                string  `json:"name"`
	LoyaltySteps        uint8   `json:"loyaltySteps"`
	LoyaltyPointsId     uint8   `json:"loyaltyPointsId"`
	LoyaltyPointsName   string  `json:"loyaltyPointsName"`
	LoyaltyImagine      bool    `json:"loyaltyImagine"`
	PartialCompensation bool    `json:"partialCompensation"`
}

type LoyaltyBonus struct {
	Description      string        `json:"description"`
	Icon             string        `json:"icon"`
	LoyaltyType      string        `json:"loyaltyType"`
	Amount           LoyaltyAmount `json:"amount"`
	CompensationType string        `json:"compensationType"`
}

type Region struct {
	Country    *string `json:"country,omitempty"`
	City       *string `json:"city,omitempty"`
	Address    *string `json:"address,omitempty"`
	Zip        *string `json:"zip,omitempty"`
	AddressRus *string `json:"addressRus,omitempty"`
}

type Merchant struct {
	Name   string  `json:"name"`
	Region *Region `json:"region,omitempty"`
}

type SpendingCategory struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Brand struct {
	Name          string  `json:"name"`
	BaseTextColor *string `json:"baseTextColor,omitempty"`
	Logo          *string `json:"logo,omitempty"`
	Id            string  `json:"id"`
	RoundedLogo   bool    `json:"roundedLogo"`
	BaseColor     *string `json:"baseColor,omitempty"`
	LogoFile      *string `json:"logoFile,omitempty"`
	Link          *string `json:"link,omitempty"`
	SvgLogoFile   *string `json:"svgLogoFile"`
}

type AdditionalInfo struct {
	FieldName  string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

type LoyaltyPaymentAmount struct {
	LoyaltyAmount
	Price float64 `json:"price"`
}

type LoyaltyPayment struct {
	Amount   LoyaltyPaymentAmount `json:"amount"`
	Status   string               `json:"status"`
	SoldTime *Milliseconds        `json:"soldTime"`
}

type LoyaltyBonusSummary struct {
	Amount float64 `json:"amount"`
}

type Payment struct {
	SourceIsQr         bool           `json:"sourceIsQr"`
	BankAccountId      string         `json:"bankAccountId"`
	PaymentId          string         `json:"paymentId"`
	ProviderGroupId    *string        `json:"providerGroupId,omitempty"`
	PaymentType        string         `json:"paymentType"`
	FeeAmount          *MoneyAmount   `json:"feeAmount,omitempty"`
	ProviderId         string         `json:"providerId"`
	HasPaymentOrder    bool           `json:"hasPaymentOrder"`
	Comment            string         `json:"comment"`
	IsQrPayment        bool           `json:"isQrPayment"`
	FieldsValues       map[string]any `json:"fieldsValues"`
	Repeatable         bool           `json:"repeatable"`
	CardNumber         string         `json:"cardNumber"`
	TemplateId         *string        `json:"templateId,omitempty"`
	TemplateIsFavorite *bool          `json:"templateIsFavorite,omitempty"`
}

type Subgroup struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Operation struct {
	IsDispute              bool                 `json:"isDispute"`
	IsOffline              bool                 `json:"isOffline"`
	HasStatement           bool                 `json:"hasStatement"`
	IsSuspicious           bool                 `json:"isSuspicious"`
	AuthorizationId        *string              `json:"authorizationId,omitempty"`
	IsInner                bool                 `json:"isInner"`
	Id                     string               `json:"id"`
	Status                 string               `json:"status"`
	OperationTransferred   bool                 `json:"operationTransferred"`
	IdSourceType           string               `json:"idSourceType"`
	HasShoppingReceipt     *bool                `json:"hasShoppingReceipt,omitempty"`
	Type                   string               `json:"type"`
	Locations              []Location           `json:"locations,omitempty"`
	LoyaltyBonus           []LoyaltyBonus       `json:"loyaltyBonus,omitempty"`
	CashbackAmount         MoneyAmount          `json:"cashbackAmount"`
	AuthMessage            *string              `json:"authMessage,omitempty"`
	Description            string               `json:"description"`
	IsTemplatable          bool                 `json:"isTemplatable"`
	Cashback               float64              `json:"cashback"`
	Brand                  *Brand               `json:"brand,omitempty"`
	Amount                 MoneyAmount          `json:"amount"`
	OperationTime          Milliseconds         `json:"operationTime"`
	SpendingCategory       SpendingCategory     `json:"spendingCategory"`
	IsHce                  bool                 `json:"isHce"`
	Mcc                    uint                 `json:"mcc"`
	Category               Category             `json:"category"`
	AdditionalInfo         []AdditionalInfo     `json:"additionalInfo,omitempty"`
	VirtualPaymentType     uint8                `json:"virtualPaymentType"`
	Account                string               `json:"account"`
	Ucid                   *string              `json:"ucid,omitempty"`
	Merchant               *Merchant            `json:"merchant,omitempty"`
	Card                   *string              `json:"card,omitempty"`
	LoyaltyPayment         []LoyaltyPayment     `json:"loyaltyPayment,omitempty"`
	TrancheCreationAllowed bool                 `json:"trancheCreationAllowed"`
	Group                  *string              `json:"group,omitempty"`
	MccString              string               `json:"mccString"`
	CardPresent            bool                 `json:"cardPresent"`
	IsExternalCard         bool                 `json:"isExternalCard"`
	CardNumber             *string              `json:"cardNumber,omitempty"`
	AccountAmount          MoneyAmount          `json:"accountAmount"`
	LoyaltyBonusSummary    *LoyaltyBonusSummary `json:"loyaltyBonusSummary,omitempty"`
	TypeSerno              *uint                `json:"typeSerno"`
	Payment                *Payment             `json:"payment,omitempty"`
	OperationPaymentType   *string              `json:"operationPaymentType,omitempty"`
	Subgroup               *Subgroup            `json:"subgroup,omitempty"`
	DebitingTime           *Milliseconds        `json:"debitingTime,omitempty"`
	PosId                  *string              `json:"posId,omitempty"`
	Subcategory            *string              `json:"subcategory,omitempty"`
	SenderAgreement        *string              `json:"senderAgreement,omitempty"`
	PointOfSaleId          *uint64              `json:"pointOfSaleId,omitempty"`
	Compensation           *string              `json:"compensation,omitempty"`
	InstallmentStatus      *string              `json:"installmentStatus,omitempty"`
	SenderDetails          *string              `json:"senderDetails,omitempty"`
	PartnerType            *string              `json:"partnerType,omitempty"`
	Nomination             *string              `json:"nomination,omitempty"`
	Message                *string              `json:"message,omitempty"`
	TrancheId              *string              `json:"trancheId,omitempty"`
}

type OperationsOut = []Operation

type ShoppingReceiptIn struct {
	OperationId   string     `url:"operationId" validate:"required"`
	OperationTime *time.Time `url:"operationTime,unixmilli,omitempty"`
	IdSourceType  *string    `url:"idSourceType,omitempty"`
	Account       *string    `url:"account,omitempty"`
}

func (in ShoppingReceiptIn) auth() auth                  { return force }
func (in ShoppingReceiptIn) path() string                { return shoppingReceiptPath }
func (in ShoppingReceiptIn) out() (_ ShoppingReceiptOut) { return }
func (in ShoppingReceiptIn) exprc() string               { return "OK" }

type ReceiptItem struct {
	Name     string   `json:"name"`
	Price    float64  `json:"price"`
	Sum      float64  `json:"sum"`
	Quantity float64  `json:"quantity"`
	NdsRate  *uint8   `json:"ndsRate"`
	Nds      *uint8   `json:"nds"`
	Nds10    *float64 `json:"nds10,omitempty"`
	Nds18    *float64 `json:"nds18,omitempty"`
	BrandId  int64    `json:"brand_id"`
	GoodId   int64    `json:"good_id"`
}

type Receipt struct {
	RetailPlace             *string       `json:"retailPlace,omitempty"`
	RetailPlaceAddress      *string       `json:"retailPlaceAddress,omitempty"`
	CreditSum               *float64      `json:"creditSum,omitempty"`
	ProvisionSum            *float64      `json:"provisionSum,omitempty"`
	FiscalDriveNumber       *uint64       `json:"fiscalDriveNumber,omitempty"`
	OperationType           uint8         `json:"operationType"`
	CashTotalSum            float64       `json:"cashTotalSum"`
	ShiftNumber             uint          `json:"shiftNumber"`
	KktRegId                string        `json:"kktRegId"`
	Items                   []ReceiptItem `json:"items"`
	TotalSum                float64       `json:"totalSum"`
	EcashTotalSum           float64       `json:"ecashTotalSum"`
	Nds10                   *float64      `json:"nds10,omitempty"`
	Nds18                   *float64      `json:"nds18,omitempty"`
	UserInn                 string        `json:"userInn"`
	DateTime                Seconds       `json:"dateTime"`
	TaxationType            uint8         `json:"taxationType"`
	PrepaidSum              *float64      `json:"prepaidSum,omitempty"`
	FiscalSign              uint64        `json:"fiscalSign"`
	RequestNumber           uint          `json:"requestNumber"`
	Operator                *string       `json:"operator,omitempty"`
	AppliedTaxationType     uint8         `json:"appliedTaxationType"`
	FiscalDocumentNumber    uint64        `json:"fiscalDocumentNumber"`
	User                    *string       `json:"user,omitempty"`
	FiscalDriveNumberString string        `json:"fiscalDriveNumberString"`
}

type ShoppingReceiptOut struct {
	OperationDateTime Milliseconds `json:"operationDateTime"`
	OperationId       string       `json:"operationId"`
	Receipt           Receipt      `json:"receipt"`
}
