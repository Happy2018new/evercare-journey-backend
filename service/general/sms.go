package general

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/patrickmn/go-cache"
)

// map[AccountPhone]SMSTransaction
var cachedSMSTransaction = cache.New(time.Minute*5, time.Minute)
var smsTransactionMu sync.Mutex

const (
	SmsConsumeStatusSuccess uint8 = iota
	SmsConsumeStatusExpired
	SmsConsumeStatusMismatch
	SmsConsumeStatusTooManyAttempts
)

const maxSmsVerifyAttempts uint8 = 5

type SMSTransaction struct {
	phone    string
	code     string
	ctx      any
	attempts uint8
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

func OpenNewSMSTransaction(accountPhone string, ctx any) (tran *SMSTransaction, useCached bool, generalErr *define.GeneralError) {
	accountPhone = strings.TrimSpace(accountPhone)
	smsTransactionMu.Lock()
	defer smsTransactionMu.Unlock()
	if value, ok := cachedSMSTransaction.Get(accountPhone); ok {
		cached, valid := value.(*SMSTransaction)
		if valid && cached != nil {
			return cached, true, nil
		}
		// A malformed cache entry must not make callers receive a nil
		// transaction while reporting success. Remove it and issue a fresh code.
		cachedSMSTransaction.Delete(accountPhone)
	}
	verifyCode, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, false, define.NewGeneralError("OpenNewSMSTransaction", err, define.LangKeyGeneralSmsGenFailErr)
	}

	tran = &SMSTransaction{
		phone: accountPhone,
		code:  fmt.Sprintf("%06d", verifyCode.Int64()),
		ctx:   ctx,
	}
	cachedSMSTransaction.Set(accountPhone, tran, cache.DefaultExpiration)
	return tran, false, nil
}

func DiscardSMSTransaction(accountPhone string) {
	smsTransactionMu.Lock()
	defer smsTransactionMu.Unlock()
	cachedSMSTransaction.Delete(strings.TrimSpace(accountPhone))
}

func ConsumeSMSTransaction(accountPhone string, verifyCode string) (status uint8, ctx any) {
	accountPhone = strings.TrimSpace(accountPhone)
	verifyCode = strings.TrimSpace(verifyCode)
	smsTransactionMu.Lock()
	defer smsTransactionMu.Unlock()
	val, ok := cachedSMSTransaction.Get(accountPhone)
	if !ok {
		return SmsConsumeStatusExpired, nil
	}

	tran, valid := val.(*SMSTransaction)
	if !valid || tran == nil {
		cachedSMSTransaction.Delete(accountPhone)
		return SmsConsumeStatusExpired, nil
	}
	if subtle.ConstantTimeCompare([]byte(tran.VerifyCode()), []byte(verifyCode)) != 1 {
		tran.attempts++
		if tran.attempts >= maxSmsVerifyAttempts {
			cachedSMSTransaction.Delete(accountPhone)
			return SmsConsumeStatusTooManyAttempts, nil
		}
		cachedSMSTransaction.Set(accountPhone, tran, cache.DefaultExpiration)
		return SmsConsumeStatusMismatch, nil
	}

	cachedSMSTransaction.Delete(accountPhone)
	return SmsConsumeStatusSuccess, tran.Context()
}
