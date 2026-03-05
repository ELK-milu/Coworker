package wechat

import "encoding/xml"

// MsgType 微信消息类型
type MsgType string

const (
	MsgTypeText  MsgType = "text"
	MsgTypeEvent MsgType = "event"
	MsgTypeImage MsgType = "image"
	MsgTypeVoice MsgType = "voice"
)

// EventType 微信事件类型
type EventType string

const (
	EventSubscribe   EventType = "subscribe"
	EventUnsubscribe EventType = "unsubscribe"
)

// InMessage 微信推送的 XML 消息
type InMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"` // 发送者 OpenID
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      MsgType  `xml:"MsgType"`
	Content      string   `xml:"Content,omitempty"`     // text 消息内容
	MsgId        int64    `xml:"MsgId,omitempty"`        // 消息 ID
	Event        EventType `xml:"Event,omitempty"`        // 事件类型
	EventKey     string   `xml:"EventKey,omitempty"`     // 事件 Key
	MediaId      string   `xml:"MediaId,omitempty"`      // 媒体 ID
	Recognition  string   `xml:"Recognition,omitempty"`  // 语音识别结果
}

// ReplyTextMessage 被动回复文本消息
type ReplyTextMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// CustomerServiceMessage 客服消息（JSON 格式，通过 API 发送）
type CustomerServiceMessage struct {
	ToUser  string         `json:"touser"`
	MsgType string         `json:"msgtype"`
	Text    *CSTextContent `json:"text,omitempty"`
}

// CSTextContent 客服文本消息内容
type CSTextContent struct {
	Content string `json:"content"`
}
