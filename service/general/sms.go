package general

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/patrickmn/go-cache"
)

// map[AccountPhone]SMSTransaction
var cachedSMSTransaction = cache.New(time.Minute*5, time.Minute)

const (
	SmsConsumeStatusSuccess uint8 = iota
	SmsConsumeStatusExpired
	SmsConsumeStatusMismatch
)

type SMSTransaction struct {
	phone string
	code  string
	ctx   any
}

func (s *SMSTransaction) AccountPhone() string {
	return s.phone
}

func (s *SMSTransaction) VerifyCode() string {
	return s.code
}

func (s *SMSTransaction) Context() any {
	return s.ctx
}

func OpenNewSMSTransaction(accountPhone string, ctx any) (tran *SMSTransaction, generalErr *define.GeneralError) {
	if _, ok := cachedSMSTransaction.Get(accountPhone); ok {
		return nil, define.NewGeneralError(
			"OpenNewSMSTransaction",
			fmt.Errorf("SMS transaction for account phone %s is already exists", accountPhone),
			define.LangKeyGeneralSmsTranBusyErr,
		)
	}

	verifyCode, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, define.NewGeneralError("OpenNewSMSTransaction", err, define.LangKeyGeneralSmsGenFailErr)
	}

	tran = &SMSTransaction{
		phone: accountPhone,
		code:  fmt.Sprintf("%06d", verifyCode.Int64()),
		ctx:   ctx,
	}
	cachedSMSTransaction.Set(accountPhone, tran, cache.DefaultExpiration)
	return tran, nil
}

func DiscardSMSTransaction(accountPhone string) {
	cachedSMSTransaction.Delete(accountPhone)
}

func ConsumeSMSTransaction(accountPhone string, verifyCode string) (status uint8, ctx any) {
	val, ok := cachedSMSTransaction.Get(accountPhone)
	if !ok {
		return SmsConsumeStatusExpired, nil
	}

	tran := val.(*SMSTransaction)
	if tran.VerifyCode() != verifyCode {
		return SmsConsumeStatusMismatch, nil
	}

	DiscardSMSTransaction(accountPhone)
	return SmsConsumeStatusSuccess, tran.Context()
}
