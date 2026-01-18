package gameserver

const MSG_TYPE_SMS = "SMS"
const MSG_TYPE_EVENT = "EVENT"
const MSG_TYPE_NOTICE = "NOTICE"

type Message struct {
	MsgType string `json:"type"`
	MsgBody any    `json:"body"`
}

type SMSMesssageBody struct {
	Data string `json:"data"`
}
