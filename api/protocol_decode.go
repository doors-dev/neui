package api

import (
	"encoding/json"
	"fmt"
)

// Contents decodes any content block by its "type" tag. A new content type must
// also be added to decodeContent, or stored threads holding it stop loading.
type Contents []Content

func (cs *Contents) UnmarshalJSON(data []byte) error {
	raws, err := rawContents(data)
	if err != nil {
		return err
	}
	list := make(Contents, 0, len(raws))
	for _, raw := range raws {
		c, err := decodeContent(raw)
		if err != nil {
			return err
		}
		list = append(list, c)
	}
	*cs = list
	return nil
}

// UserContents decodes the narrower set a user may author, which is also what
// a tool result carries. A block that decodes but does not implement
// UserContent is an error, never a silent drop.
type UserContents []UserContent

func (cs *UserContents) UnmarshalJSON(data []byte) error {
	raws, err := rawContents(data)
	if err != nil {
		return err
	}
	list := make(UserContents, 0, len(raws))
	for _, raw := range raws {
		c, err := decodeContent(raw)
		if err != nil {
			return err
		}
		uc, ok := c.(UserContent)
		if !ok {
			return fmt.Errorf("content %q is not user content", c.contentType())
		}
		list = append(list, uc)
	}
	*cs = list
	return nil
}

func rawContents(data []byte) ([]json.RawMessage, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	return raws, nil
}

func decodeContent(raw json.RawMessage) (Content, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case "text":
		return decodeAs[ContentText](raw)
	case "image":
		return decodeAs[ContentImage](raw)
	case "document":
		return decodeAs[ContentDocument](raw)
	case "tool_use":
		return decodeAs[ContentToolUse](raw)
	case "tool_result":
		return decodeAs[ContentToolResult](raw)
	case "reasoning":
		return decodeAs[ContentReasoning](raw)
	case "opaque":
		return decodeAs[ContentOpaque](raw)
	case "":
		return nil, fmt.Errorf("content has no type")
	default:
		return nil, fmt.Errorf("unknown content type %q", probe.Type)
	}
}

func decodeAs[T Content](raw json.RawMessage) (Content, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}
