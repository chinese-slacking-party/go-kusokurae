package main

const SUCC_CODE = 0
const SUCC_MSG = "success"
const COMMON_ERR_CODE = 1

type BaseRes struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data any    `json:"data"`
}

func NewErrorRes(code int, msg string) *BaseRes {
	return &BaseRes{
		Code: code,
		Msg:  msg,
		Data: nil,
	}
}

func NewSuccessRes(data any) *BaseRes {
	return &BaseRes{
		Code: SUCC_CODE,
		Msg:  SUCC_MSG,
		Data: data,
	}
}
