package utils

import (
	"fmt"
	"math/rand"

	"github.com/golang/freetype/truetype"
	"github.com/wenlng/go-captcha-assets/bindata/chars"
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/shapes"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

const (
	PaddingValidateTextCaptcha   = 0
	PaddingValidateSlideCaptcha  = 10
	PaddingValidateRotateCaptcha = 10
)

var (
	clickTextCaptchaV1 click.Captcha
	clickTextCaptchaV2 click.Captcha
	clickTextCaptchaV3 click.Captcha
	clickTextCaptchaV4 click.Captcha

	slideTileCaptcha slide.Captcha
	slideDropCaptcha slide.Captcha

	rotateImgCaptcha rotate.Captcha
)

type Dot [2]int

func NewDot(x int, y int) Dot {
	return Dot{x, y}
}

func (d Dot) X() int {
	return d[0]
}

func (d Dot) Y() int {
	return d[1]
}

func MakeTextCaptcha() (data click.CaptchaData, err error) {
	switch rand.Intn(4) {
	case 0:
		data, err = clickTextCaptchaV1.Generate()
	case 1:
		data, err = clickTextCaptchaV2.Generate()
	case 2:
		data, err = clickTextCaptchaV3.Generate()
	case 3:
		data, err = clickTextCaptchaV4.Generate()
	}
	if err != nil {
		return nil, fmt.Errorf("MakeTextCaptcha: %w", err)
	}
	return
}

func MakeSlideCaptcha() (data slide.CaptchaData, isDrop bool, err error) {
	switch rand.Intn(2) {
	case 0:
		data, err = slideTileCaptcha.Generate()
	case 1:
		data, err = slideDropCaptcha.Generate()
		isDrop = true
	}
	if err != nil {
		return nil, false, fmt.Errorf("MakeSlideCaptcha: %w", err)
	}
	return
}

func MakeRotateCaptcha() (data rotate.CaptchaData, err error) {
	data, err = rotateImgCaptcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("MakeRotateCaptcha: %w", err)
	}
	return
}

func ValidateTextCaptcha(captcha click.CaptchaData, answer []Dot) bool {
	dots := captcha.GetData()

	if len(dots) != len(answer) {
		return false
	}
	for index := range dots {
		src, dst := dots[index], answer[index]
		if !click.Validate(dst.X(), dst.Y(), src.X, src.Y, src.Width, src.Height, PaddingValidateTextCaptcha) {
			return false
		}
	}

	return true
}

func ValidateSlideCaptcha(captcha slide.CaptchaData, answer Dot) bool {
	block := captcha.GetData()
	return slide.Validate(answer.X(), answer.Y(), block.X, block.Y, PaddingValidateSlideCaptcha)
}

func ValidateRotateCaptcha(captcha rotate.CaptchaData, answer int) bool {
	block := captcha.GetData()
	return rotate.Validate(answer, block.Angle, PaddingValidateRotateCaptcha)
}

func init() {
	fonts, err := fzshengsksjw.GetFont()
	if err != nil {
		panic(err)
	}
	imgs, err := imagesv2.GetImages()
	if err != nil {
		panic(err)
	}
	shapeMaps, err := shapes.GetShapes()
	if err != nil {
		panic(err)
	}

	{
		builder := click.NewBuilder()
		builder.SetResources(
			click.WithChars(chars.GetChineseChars()),
			click.WithFonts([]*truetype.Font{fonts}),
			click.WithBackgrounds(imgs),
		)
		clickTextCaptchaV1 = builder.Make()

		builder = click.NewBuilder()
		builder.SetResources(
			click.WithChars(chars.GetAlphaChars()),
			click.WithFonts([]*truetype.Font{fonts}),
			click.WithBackgrounds(imgs),
		)
		clickTextCaptchaV2 = builder.Make()

		builder = click.NewBuilder()
		builder.SetResources(
			click.WithChars(chars.GetMixinAlphaChars()),
			click.WithFonts([]*truetype.Font{fonts}),
			click.WithBackgrounds(imgs),
		)
		clickTextCaptchaV3 = builder.Make()

		builder = click.NewBuilder()
		builder.SetResources(
			click.WithShapes(shapeMaps),
			click.WithFonts([]*truetype.Font{fonts}),
			click.WithBackgrounds(imgs),
		)
		clickTextCaptchaV4 = builder.Make()
	}

	{
		graphs, err := tiles.GetTiles()
		if err != nil {
			panic(err)
		}

		var newGraphs = make([]*slide.GraphImage, 0, len(graphs))
		for i := range graphs {
			graph := graphs[i]
			newGraphs = append(newGraphs, &slide.GraphImage{
				OverlayImage: graph.OverlayImage,
				MaskImage:    graph.MaskImage,
				ShadowImage:  graph.ShadowImage,
			})
		}

		builder := slide.NewBuilder(
			slide.WithGenGraphNumber(2),
		)
		builder.SetResources(
			slide.WithGraphImages(newGraphs),
			slide.WithBackgrounds(imgs),
		)
		slideTileCaptcha = builder.Make()

		builder = slide.NewBuilder(
			slide.WithGenGraphNumber(2),
			slide.WithEnableGraphVerticalRandom(true),
		)
		builder.SetResources(
			slide.WithGraphImages(newGraphs),
			slide.WithBackgrounds(imgs),
		)
		slideDropCaptcha = builder.MakeDragDrop()
	}

	{
		builder := rotate.NewBuilder()
		builder.SetResources(rotate.WithImages(imgs))
		rotateImgCaptcha = builder.Make()
	}
}
