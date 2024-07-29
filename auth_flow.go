package tinkoff

import (
	"context"

	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
)

func withSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, Session{}, session)
}

func getSession(ctx context.Context) *Session {
	if session, ok := ctx.Value(Session{}).(*Session); ok {
		return session
	}

	return nil
}

// AuthFlow обозначает способ аутентификации.
type AuthFlow interface {
	authorize(ctx context.Context, client *Client, authorizer Authorizer) (*Session, error)
}

// ApiAuthFlow производит аутентификацию с помощью вызовов API.
// В начале июня 2024 Тинькофф что-то поменял, и теперь не срабатывает подтверждение СМС-кодом (не хватает каких-то полей).
// Несмотря на то, что этот способ аутентификации сейчас не работает, он все еще остается дефолтным в клиенте для обратной
// совместимости, и все еще остается в коде на случай потенциальной починки в будущем.
var ApiAuthFlow AuthFlow = &apiAuthFlow{}

type apiAuthFlow struct{}

func (f *apiAuthFlow) authorize(ctx context.Context, c *Client, authorizer Authorizer) (*Session, error) {
	var session *Session
	if resp, err := executeCommon(ctx, c, sessionIn{}); err != nil {
		return nil, errors.Wrap(err, "get new sessionid")
	} else {
		session = &Session{ID: resp.Payload}
		ctx = withSession(ctx, session)
	}

	if resp, err := executeCommon(ctx, c, phoneSignUpIn{Phone: c.credential.Phone}); err != nil {
		return nil, errors.Wrap(err, "phone sign up")
	} else {
		code, err := authorizer.GetConfirmationCode(ctx, c.credential.Phone)
		if err != nil {
			return nil, errors.Wrap(err, "get confirmation code")
		}

		if _, err := executeCommon(ctx, c, confirmIn{
			InitialOperation:       "sign_up",
			InitialOperationTicket: resp.OperationTicket,
			ConfirmationData:       confirmationData{SMSBYID: code},
		}); err != nil {
			return nil, errors.Wrap(err, "submit confirmation code")
		}
	}

	if _, err := executeCommon(ctx, c, passwordSignUpIn{Password: c.credential.Password}); err != nil {
		return nil, errors.Wrap(err, "password sign up")
	}

	if _, err := executeCommon(ctx, c, levelUpIn{}); err != nil {
		return nil, errors.Wrap(err, "level up")
	}

	return session, nil
}

// SeleniumAuthFlow производит аутентификацию с помощью Selenium.
// Вероятно, самый оптимальный способ с точки зрения поддержки и дальнейших доработок.
type SeleniumAuthFlow struct {
	Capabilities selenium.Capabilities
	URLPrefix    string
}

func (f *SeleniumAuthFlow) authorize(ctx context.Context, c *Client, authorizer Authorizer) (*Session, error) {
	driver, err := selenium.NewRemote(f.Capabilities, f.URLPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "create remote")
	}

	defer driver.Close()

	if err := driver.MaximizeWindow(""); err != nil {
		return nil, errors.Wrap(err, "maximize window")
	}

	if err := driver.Get(baseURL + "/login"); err != nil {
		return nil, errors.Wrap(err, "open login page")
	}

	var complete bool
	steps := map[string]func(el selenium.WebElement) error{
		"//input[@automation-id='phone-input']":    func(el selenium.WebElement) error { return el.SendKeys(c.credential.Phone + selenium.EnterKey) },
		"//input[@automation-id='password-input']": func(el selenium.WebElement) error { return el.SendKeys(c.credential.Password + selenium.EnterKey) },
		"//input[@automation-id='otp-input']": func(el selenium.WebElement) error {
			code, err := authorizer.GetConfirmationCode(ctx, c.credential.Phone)
			if err != nil {
				return errors.Wrap(err, "get confirmation code")
			}

			return el.SendKeys(code)
		},
		"//button[@automation-id='cancel-button']": func(el selenium.WebElement) error { return el.Click() },
		"//a[@href='/new-product/']":               func(_ selenium.WebElement) error { complete = true; return nil },
	}

	for !complete {
		if err := driver.Wait(func(wd selenium.WebDriver) (bool, error) {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			default:
			}

			for xpath, handle := range steps {
				elements, err := driver.FindElements(selenium.ByXPATH, xpath)
				if err != nil {
					return false, errors.Wrapf(err, "find xpath '%s'", xpath)
				}

				for _, element := range elements {
					displayed, err := element.IsDisplayed()
					if err != nil {
						return false, errors.Wrapf(err, "xpath '%s' is displayed", xpath)
					}

					if displayed {
						if err := handle(element); err != nil {
							return false, errors.Wrapf(err, "handle xpath '%s'", xpath)
						}

						delete(steps, xpath)
						return true, nil
					}
				}
			}

			return false, nil
		}); err != nil {
			return nil, err
		}
	}

	sessionID, err := driver.GetCookie("api_session")
	if err != nil {
		return nil, errors.Wrap(err, "session cookie not found")
	}

	return &Session{
		ID: sessionID.Value,
	}, nil
}
