package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const nameLimit = 80

const moduleDesc = `The conversation this runs in; its name is what the user sees in the chat list and in the header above the messages.`
const nameDesc = `Name the chat. Call it only when a system instruction asks for a name, or when the user asks for one; never rename on your own. Replaces the previous name.`

func Module(context.Context) common.Module {
	m := tools.NewModule("chat", moduleDesc)
	if _, err := m.Add("name", nameDesc, nameTool); err != nil {
		panic(err)
	}
	return m
}

type NameArg struct {
	Name string `json:"name" required:"true" description:"the new name: 2–6 words of plain text naming the topic, not a sentence; no quotes, markup or line breaks"`
}

func nameTool(ctx context.Context, arg NameArg) (struct{}, error) {
	name := strings.TrimSpace(arg.Name)
	if name == "" {
		return struct{}{}, errors.New("name is empty: pass a short title naming the topic")
	}
	if strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return struct{}{}, errors.New("name must be a single line")
	}
	if utf8.RuneCountInString(name) > nameLimit {
		return struct{}{}, fmt.Errorf("name must be at most %d characters, shorten it to a few words", nameLimit)
	}
	return struct{}{}, common.GetController(ctx).Rename(ctx, name)
}
