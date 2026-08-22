package define

import "fmt"

type GeneralError struct {
	error
	debugInfo string
	publicMsg *LangFormat
	extraData any
}

func NewGeneralError(source string, err error, publicMsg ...string) *GeneralError {
	g := &GeneralError{
		error:     err,
		debugInfo: fmt.Sprintf("%v", err),
	}
	switch len(publicMsg) {
	case 0:
		g.publicMsg = NewLangFormat(LangKeyGeneralUnknownErr)
	case 1:
		g.publicMsg = NewLangFormat(publicMsg[0])
	default:
		g.publicMsg = NewLangFormat(publicMsg[0], publicMsg[1:]...)
	}
	return g.AppendSource(source)
}

func (g *GeneralError) Error() string {
	return g.debugInfo
}

func (g *GeneralError) Unwrap() error {
	return g.error
}

func (g *GeneralError) GetPublicMsg() *LangFormat {
	return g.publicMsg
}

func (g *GeneralError) SetPublicMsg(key string, args ...string) *GeneralError {
	g.publicMsg.LangKey = key
	g.publicMsg.LangArgs = args
	return g
}

func (g *GeneralError) GetExtraData() any {
	return g.extraData
}

func (g *GeneralError) SetExtraData(extraData any) *GeneralError {
	g.extraData = extraData
	return g
}

func (g *GeneralError) AppendSource(source string) *GeneralError {
	if len(source) > 0 {
		g.debugInfo = fmt.Sprintf("%s: %v", source, g.debugInfo)
	}
	return g
}
