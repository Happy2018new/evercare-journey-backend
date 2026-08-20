package define

import "fmt"

type GeneralError struct {
	error
	debugInfo string
	publicMsg string
	extraData any
}

func NewGeneralError(err error, source string, publicMsg string) *GeneralError {
	g := &GeneralError{
		error:     err,
		debugInfo: fmt.Sprintf("%v", err),
		publicMsg: publicMsg,
	}
	return g.AppendSource(source)
}

func (g *GeneralError) Error() string {
	return g.debugInfo
}

func (g *GeneralError) Unwrap() error {
	return g.error
}

func (g *GeneralError) GetPublicMsg() string {
	return g.publicMsg
}

func (g *GeneralError) SetPublicMsg(publicMsg string) *GeneralError {
	g.publicMsg = publicMsg
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
