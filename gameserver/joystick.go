package gameserver

type Joystick interface {
	ReadMessage() (Message, error)
	WriteMessage(Message) error
}
