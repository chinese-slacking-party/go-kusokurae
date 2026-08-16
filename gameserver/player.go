package gameserver

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxNicknameRunes = 20

var (
	ErrNicknameRequired = errors.New("nickname is required")
	ErrNicknameTooLong  = errors.New("nickname too long: max 20 characters")
	ErrNicknameInvalid  = errors.New("nickname contains invalid characters")
)

type Player struct {
	ID           string
	Nickname     string
	RoomID       string
	RoomPosition int32
	NoticeCh     chan Message
	OperatorCh   chan Message
}

// ValidateNickname checks a raw nickname input: non-empty after trim,
// 1..MaxNicknameRunes runes, printable Unicode only (no control characters).
func ValidateNickname(nickname string) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return ErrNicknameRequired
	}
	if utf8.RuneCountInString(nickname) > MaxNicknameRunes {
		return ErrNicknameTooLong
	}
	for _, r := range nickname {
		if !unicode.IsPrint(r) {
			return ErrNicknameInvalid
		}
	}
	return nil
}

// NewPlayer creates a player with a validated nickname (stored trimmed).
func NewPlayer(nickname string) (*Player, error) {
	if err := ValidateNickname(nickname); err != nil {
		return nil, err
	}
	u, err := uuid.NewRandom()
	if err != nil {
		panic("failed to generate player ID")
	}
	return &Player{
		ID:           u.String(),
		Nickname:     strings.TrimSpace(nickname),
		RoomID:       "",
		RoomPosition: -1,
		NoticeCh:     make(chan Message),
		OperatorCh:   make(chan Message),
	}, nil
}

func (p *Player) Sit(roomID string, roomPosition int32) {
	p.RoomID = roomID
	p.RoomPosition = roomPosition
}
