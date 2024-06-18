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

type authFlow interface {
	authorize(ctx context.Context, client *Client, authorizer Authorizer) (*Session, error)
}

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

type SeleniumAuthFlow struct {
	Capabilities selenium.Capabilities
}

type seleniumAuthStep uint8

const (
	seleniumAuthPhoneInput seleniumAuthStep = iota
	seleniumAuthOTPInput
	seleniumAuthPasswordInput
	seleniumAuthAccessCode
	seleniumAuthComplete
)

func (s seleniumAuthStep) xpath() string {
	switch s {
	case seleniumAuthPhoneInput:
		return "//input[@name='phone']"
	case seleniumAuthOTPInput:
		return "//input[@name='otp']"
	case seleniumAuthPasswordInput:
		return "//input[@name='password']"
	case seleniumAuthAccessCode:
		return "//button[@automation-id='cancel-button']"
	case seleniumAuthComplete:
		return "//a[@href='/new-product/']"
	default:
		panic("unimplemented")
	}
}

type seleniumAuthStepElement struct {
	step    seleniumAuthStep
	element selenium.WebElement
}

func (f *SeleniumAuthFlow) authorize(ctx context.Context, c *Client, authorizer Authorizer) (*Session, error) {
	driver, err := selenium.NewRemote(f.Capabilities, "")
	if err != nil {
		return nil, errors.Wrap(err, "create remote")
	}

	defer driver.Close()

	if err := driver.MaximizeWindow(""); err != nil {
		return nil, errors.Wrap(err, "maximize window")
	}

	if err := driver.Get("https://tinkoff.ru/auth/login"); err != nil {
		return nil, errors.Wrap(err, "open login page")
	}

	steps := map[seleniumAuthStep]bool{
		seleniumAuthPhoneInput:    true,
		seleniumAuthOTPInput:      true,
		seleniumAuthPasswordInput: true,
		seleniumAuthAccessCode:    true,
		seleniumAuthComplete:      true,
	}

	for {
		var se seleniumAuthStepElement
		if err := driver.Wait(func(wd selenium.WebDriver) (bool, error) {
			return f.detectStep(ctx, wd, steps, &se)
		}); err != nil {
			return nil, errors.Wrap(err, "detect step element")
		}

		switch se.step {
		case seleniumAuthPhoneInput:
			if err := se.element.SendKeys(c.credential.Phone + selenium.EnterKey); err != nil {
				return nil, errors.Wrap(err, "input phone")
			}
		case seleniumAuthPasswordInput:
			if err := se.element.SendKeys(c.credential.Password + selenium.EnterKey); err != nil {
				return nil, errors.Wrap(err, "input password")
			}
		case seleniumAuthOTPInput:
			code, err := authorizer.GetConfirmationCode(ctx, c.credential.Phone)
			if err != nil {
				return nil, errors.Wrap(err, "get confirmation code")
			}

			if err := se.element.SendKeys(code); err != nil {
				return nil, errors.Wrap(err, "input otp")
			}
		case seleniumAuthAccessCode:
			if err := se.element.Click(); err != nil {
				return nil, errors.Wrap(err, "click access code cancel button")
			}
		case seleniumAuthComplete:
			sessionID, err := driver.GetCookie("api_session")
			if err != nil {
				return nil, errors.Wrap(err, "session cookie not found")
			}

			return &Session{
				ID: sessionID.Value,
			}, nil
		default:
			panic("unimplemented")
		}

		delete(steps, se.step)
	}
}

func (f *SeleniumAuthFlow) detectStep(ctx context.Context, driver selenium.WebDriver, steps map[seleniumAuthStep]bool, se *seleniumAuthStepElement) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	for step := range steps {
		elements, err := driver.FindElements(selenium.ByXPATH, step.xpath())
		if err != nil {
			return false, err
		}

		for _, el := range elements {
			displayed, err := el.IsDisplayed()
			if err != nil {
				return false, errors.Wrap(err, "get IsDisplayed")
			}

			if displayed {
				se.step = step
				se.element = el
				return true, nil
			}
		}
	}

	return false, nil
}
