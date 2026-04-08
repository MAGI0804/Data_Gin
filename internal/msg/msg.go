package msg

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translation "github.com/go-playground/validator/v10/translations/en"
	zh_translation "github.com/go-playground/validator/v10/translations/zh"
)

type Response struct {
	Code int             `json:"code"`
	Msg  any             `json:"msg"`
	Data *map[string]any `json:"data"`
}

type ErrResponseST struct {
	Code int             `json:"code"`
	Msg  any             `json:"msg"`
	Data *map[string]any `json:"data"`
	Err  any             `json:err`
}

var trans ut.Translator

func initTranslator(language string) error {
	//转换成go-playground的validator
	validate, ok := binding.Validator.Engine().(*validator.Validate)
	if ok {
		//创建翻译器
		zhT := zh.New()
		enT := en.New()

		//创建通用翻译器
		//第一个参数是备用语言，后面的是应当支持的语言
		uni := ut.New(enT, enT, zhT)

		//从通过中获取指定语言翻译器
		trans, ok = uni.GetTranslator(language)
		if !ok {
			return fmt.Errorf("not found translator %s", language)
		}

		//绑定到gin的验证器上，对binding的tag进行翻译
		switch language {
		case "zh":
			err := zh_translation.RegisterDefaultTranslations(validate, trans)
			if err != nil {
				return err
			}
		default:
			err := en_translation.RegisterDefaultTranslations(validate, trans)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func remove(errors map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range errors {
		result[key[strings.Index(key, ".")+1:]] = value
	}
	return result
}

func SuccessResponse(msg string, dataPtr *map[string]any) *Response {
	if dataPtr == nil {
		emptyMap := make(map[string]any)
		dataPtr = &emptyMap
	}
	return &Response{
		Code: 200,
		Msg:  msg,
		Data: dataPtr,
	}
}

func SuccessResponseStr(msg string) *Response {

	return &Response{
		Code: 200,
		Msg:  msg,
		Data: &map[string]any{},
	}
}

func ErrResponse(msg string, errors error) *ErrResponseST {
	err := initTranslator("zh")
	if err != nil {
		panic(err)
	}
	B := errors.Error()
	if errors, ok := err.(validator.ValidationErrors); ok {
		B := remove(errors.Translate(trans))
		return &ErrResponseST{
			Code: 201,
			Msg:  msg,
			Data: &map[string]any{},
			Err:  B,
		}
	}
	return &ErrResponseST{
		Code: 201,
		Msg:  msg,
		Data: &map[string]any{},
		Err:  B,
	}
}

func ErrResponseStr(msg string) *ErrResponseST {

	return &ErrResponseST{
		Code: 201,
		Msg:  msg,
		Data: &map[string]any{},
		Err:  "",
	}
}
