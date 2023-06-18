package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"github.com/davecgh/go-spew/spew"
	"github.com/jfk9w-go/based"
	"github.com/jfk9w-go/tinkoff-api"
	"github.com/pkg/errors"
)

type jsonSessionStorage struct {
	path string
}

func (s jsonSessionStorage) LoadSession(ctx context.Context, phone string) (*tinkoff.Session, error) {
	file, err := s.open(os.O_RDONLY)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	defer file.Close()
	contents := make(map[string]tinkoff.Session)
	if err := json.NewDecoder(file).Decode(&contents); err != nil {
		return nil, errors.Wrap(err, "decode json")
	}

	if session, ok := contents[phone]; ok {
		return &session, nil
	}

	return nil, nil
}

func (s jsonSessionStorage) UpdateSession(ctx context.Context, phone string, session *tinkoff.Session) error {
	file, err := s.open(os.O_RDWR | os.O_CREATE)
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return errors.Wrap(err, "stat")
	}

	contents := make(map[string]tinkoff.Session)
	if stat.Size() > 0 {
		if err := json.NewDecoder(file).Decode(&contents); err != nil {
			return errors.Wrap(err, "decode json")
		}
	}

	if session != nil {
		contents[phone] = *session
	} else {
		delete(contents, phone)
	}

	if err := file.Truncate(0); err != nil {
		return errors.Wrap(err, "truncate file")
	}

	if _, err := file.Seek(0, 0); err != nil {
		return errors.Wrap(err, "seek to the start of file")
	}

	if err := json.NewEncoder(file).Encode(&contents); err != nil {
		return errors.Wrap(err, "encode json")
	}

	return nil
}

func (s jsonSessionStorage) open(flag int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), os.ModeDir); err != nil {
		return nil, errors.Wrap(err, "create parent directory")
	}

	file, err := os.OpenFile(s.path, flag, 0644)
	if err != nil {
		return nil, errors.Wrap(err, "open file")
	}

	return file, nil
}

type stdinConfirmationProvider struct{}

func (p stdinConfirmationProvider) GetConfirmationCode(ctx context.Context, phone string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter confirmation code for %s: ", phone)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.Wrap(err, "read line from stdin")
	}

	return strings.Trim(text, " \n\t\v"), nil
}

func main() {
	var config struct {
		Phone        string `env:"TINKOFF_PHONE,required"`
		Password     string `env:"TINKOFF_PASSWORD,required"`
		SessionsFile string `env:"TINKOFF_SESSIONS_FILE,required"`
	}

	if err := env.Parse(&config); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := tinkoff.ClientBuilder{
		Clock: based.StandardClock,
		Credential: tinkoff.Credential{
			Phone:    config.Phone,
			Password: config.Password,
		},
		ConfirmationProvider: stdinConfirmationProvider{},
		SessionStorage:       jsonSessionStorage{path: config.SessionsFile},
	}.Build(ctx)

	if err != nil {
		panic(err)
	}

	defer client.Close()

	investOperationTypes, err := client.InvestOperationTypes(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("found %d invest operation types\n", len(investOperationTypes.OperationsTypes))
	for _, operationType := range investOperationTypes.OperationsTypes {
		spew.Dump(operationType)
		break
	}

	investAccounts, err := client.InvestAccounts(ctx, &tinkoff.InvestAccountsIn{
		Currency: "RUB",
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("found %d invest accounts\n", investAccounts.Accounts.Count)
	for _, account := range investAccounts.Accounts.List {
		spew.Dump(account)

		investOperations, err := client.InvestOperations(ctx, &tinkoff.InvestOperationsIn{
			From:            time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			To:              time.Now(),
			Limit:           10,
			BrokerAccountId: account.BrokerAccountId,
		})

		if err != nil {
			panic(err)
		}

		fmt.Printf("found %d invest operations in invest account '%s'\n", len(investOperations.Items), account.Name)
		for _, operation := range investOperations.Items {
			spew.Dump(operation)
			break
		}

		break
	}

	accounts, err := client.AccountsLightIb(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("found %d accounts\n", len(accounts))
	if len(accounts) == 0 {
		return
	}

	for _, account := range accounts {
		spew.Dump(account)

		operations, err := client.Operations(ctx, &tinkoff.OperationsIn{
			Account: account.Id,
			Start:   time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
			End:     time.Now(),
		})

		if err != nil {
			panic(err)
		}

		fmt.Printf("found %d operations in account '%s'\n", len(operations), account.Name)
		if len(operations) == 0 {
			return
		}

		for _, operation := range operations {
			if operation.HasShoppingReceipt {
				spew.Dump(operation)

				receipt, err := client.ShoppingReceipt(ctx, &tinkoff.ShoppingReceiptIn{
					OperationId: operation.Id,
				})

				switch {
				case errors.Is(err, tinkoff.ErrNoDataFound):
					continue
				case err != nil:
					panic(err)
				}

				for _, item := range receipt.Receipt.Items {
					fmt.Println(item.Name)
				}

				spew.Dump(receipt)

				break
			}
		}

		break
	}
}
