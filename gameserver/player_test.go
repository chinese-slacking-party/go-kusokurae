package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNickname(t *testing.T) {
	cases := []struct {
		name     string
		nickname string
		wantErr  bool
	}{
		{"valid ascii", "Alice", false},
		{"valid chinese", "小明", false},
		{"valid mixed", "玩家-01", false},
		{"valid with inner spaces", "Alice Bob", false},
		{"valid surrounded by spaces", "  Alice  ", false},
		{"20 runes ok", "一二三四五六七八九十一二三四五六七八九十", false},
		{"empty", "", true},
		{"only spaces", "   ", true},
		{"21 runes", "一二三四五六七八九十一二三四五六七八九十一", true},
		{"tab control char", "a\tb", true},
		{"newline control char", "a\nb", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNickname(tc.nickname)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewPlayer_ValidNicknameTrimmed(t *testing.T) {
	p, err := NewPlayer("  小明  ")
	require.NoError(t, err)
	assert.Equal(t, "小明", p.Nickname)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, int32(-1), p.RoomPosition)
}

func TestNewPlayer_InvalidNickname(t *testing.T) {
	_, err := NewPlayer("   ")
	assert.Error(t, err)
}
