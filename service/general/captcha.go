package general

import (
	"math/rand"
	"time"

	"github.com/Happy2018new/evercare-journey-backed/database/define"
	"github.com/Happy2018new/evercare-journey-backed/utils"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

// map[TransactionUUID]CaptchaTransaction
var cachedCaptchaTransaction = cache.New(time.Minute, time.Minute)

const MinConsumeTranInterval = 2

const (
	CaptchaTypeClickText uint8 = iota
	CaptchaTypeSlideTile
	CaptchaTypeSlideDrop
	CaptchaTypeRotateImg
)

const (
	CaptchaConsumeStatusSuccess uint8 = iota
	CaptchaConsumeStatusExpired
	CaptchaConsumeStatusRetry
)

type CaptchaRequest struct {
	TransactionUUID string `json:"transaction_uuid"`
	CaptchaType     uint8  `json:"captcha_type"`
	MasterImage     string `json:"master_image"`
	ThumbImage      string `json:"thumb_image"`
}

type CaptchaResponse struct {
	TransactionUUID     string      `json:"transaction_uuid"`
	TextCaptchaAnswer   []utils.Dot `json:"text_captcha_answer,omitempty"`
	SlideCaptchaAnswer  utils.Dot   `json:"slide_captcha_answer,omitempty"`
	RotateCaptchaAnswer int         `json:"rotate_captcha_answer,omitempty"`
}

type CaptchaTransaction struct {
	transactionUUID string
	captchaContext  any
	captchaType     uint8
	captchaData     any
	createUnixTime  int64
}

func (c *CaptchaTransaction) TransactionUUID() string {
	return c.transactionUUID
}

func (c *CaptchaTransaction) CaptchaContext() any {
	return c.captchaContext
}

func (c *CaptchaTransaction) CaptchaType() uint8 {
	return c.captchaType
}

func (c *CaptchaTransaction) CreateUnixTime() int64 {
	return c.createUnixTime
}

func OpenNewCaptchaTransaction(ctx any) (tran *CaptchaTransaction, generalErr *define.GeneralError) {
	var err error
	var isDrop bool
	tran = new(CaptchaTransaction)

	switch rand.Intn(3) {
	case 0:
		tran.captchaData, err = utils.MakeTextCaptcha()
		tran.captchaType = CaptchaTypeClickText
	case 1:
		tran.captchaData, isDrop, err = utils.MakeSlideCaptcha()
		tran.captchaType = CaptchaTypeSlideTile
		if isDrop {
			tran.captchaType = CaptchaTypeSlideDrop
		}
	case 2:
		tran.captchaData, err = utils.MakeRotateCaptcha()
		tran.captchaType = CaptchaTypeRotateImg
	}
	if err != nil {
		return nil, define.NewGeneralError(err, "OpenNewCaptchaTransaction", "生成图形验证码时发生未知错误")
	}

	tran.transactionUUID = uuid.NewString()
	tran.captchaContext = ctx
	tran.createUnixTime = time.Now().Unix()
	cachedCaptchaTransaction.Set(tran.transactionUUID, tran, cache.DefaultExpiration)

	return tran, nil
}

func ConsumeCaptchaTransaction(resp CaptchaResponse) (status uint8, ctx any) {
	var succ bool

	val, ok := cachedCaptchaTransaction.Get(resp.TransactionUUID)
	if !ok {
		return CaptchaConsumeStatusExpired, nil
	}
	tran := val.(*CaptchaTransaction)
	if time.Now().Unix()-tran.createUnixTime < MinConsumeTranInterval {
		return CaptchaConsumeStatusRetry, tran.captchaContext
	}

	switch tran.captchaType {
	case CaptchaTypeClickText:
		succ = utils.ValidateTextCaptcha(tran.captchaData.(click.CaptData), resp.TextCaptchaAnswer)
	case CaptchaTypeSlideTile, CaptchaTypeSlideDrop:
		succ = utils.ValidateSlideCaptcha(tran.captchaData.(slide.CaptData), resp.SlideCaptchaAnswer)
	case CaptchaTypeRotateImg:
		succ = utils.ValidateRotateCaptcha(tran.captchaData.(rotate.CaptData), resp.RotateCaptchaAnswer)
	}
	if succ {
		cachedCaptchaTransaction.Delete(resp.TransactionUUID)
		return CaptchaConsumeStatusSuccess, tran.captchaContext
	}

	return CaptchaConsumeStatusRetry, tran.captchaContext
}
