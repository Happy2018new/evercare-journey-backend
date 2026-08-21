package general

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/utils"
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
	CaptchaConsumeStatusFailed
)

type CaptchaRequest struct {
	TransactionUUID string `json:"transaction_uuid"`
	CaptchaType     uint8  `json:"captcha_type"`
	MasterImage     string `json:"master_image"`
	SecondImage     string `json:"second_image"`
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

func GenerateNewCaptchaRequest(ctx any) (*CaptchaRequest, *define.GeneralError) {
	tran, generalErr := OpenNewCaptchaTransaction(ctx)
	if generalErr != nil {
		return nil, generalErr.AppendSource("GenerateNewCaptchaRequest")
	}
	req, err := MakeCaptchaRequest(tran)
	if err != nil {
		return nil, define.NewGeneralError(err, "GenerateNewCaptchaRequest", "生成图形验证码时发生未知错误")
	}
	return req, nil
}

func DiscardCaptchaTransaction(transactionUUID string) {
	cachedCaptchaTransaction.Delete(transactionUUID)
}

func ConsumeCaptchaTransaction(resp *CaptchaResponse) (status uint8, ctx any) {
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

	DiscardCaptchaTransaction(resp.TransactionUUID)
	if !succ {
		return CaptchaConsumeStatusFailed, tran.captchaContext
	}
	return CaptchaConsumeStatusSuccess, tran.captchaContext
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

func MakeCaptchaRequest(tran *CaptchaTransaction) (request *CaptchaRequest, err error) {
	request = &CaptchaRequest{
		TransactionUUID: tran.transactionUUID,
		CaptchaType:     tran.captchaType,
	}

	switch tran.CaptchaType() {
	case CaptchaTypeClickText:
		request.MasterImage, err = tran.captchaData.(click.CaptData).GetMasterImage().ToBase64()
		request.SecondImage, err = tran.captchaData.(click.CaptData).GetThumbImage().ToBase64()
	case CaptchaTypeSlideTile, CaptchaTypeSlideDrop:
		request.MasterImage, err = tran.captchaData.(slide.CaptData).GetMasterImage().ToBase64()
		request.SecondImage, err = tran.captchaData.(slide.CaptData).GetTileImage().ToBase64()
	case CaptchaTypeRotateImg:
		request.MasterImage, err = tran.captchaData.(rotate.CaptData).GetMasterImage().ToBase64()
		request.SecondImage, err = tran.captchaData.(rotate.CaptData).GetThumbImage().ToBase64()
	}
	if err != nil {
		return nil, fmt.Errorf("MakeCaptchaRequest: %w", err)
	}

	return
}
