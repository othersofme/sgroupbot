package sgroupbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// = "https://bots.qq.com/app/getAppAccessToken"

	SgroupTarget        = "https://api.sgroup.qq.com"
	SandboxSgroupTarget = "https://sandbox.api.sgroup.qq.com"

	GatewayAPI = "/gateway"

	CreateChannelAPI = "/guilds/%s/channels"

	GetGuildListAPI = "/users/@me/guilds"

	CreateGroupMessageAPI = "/v2/groups/%s/messages" // group_openid

	CreateUserMessageAPI = "/v2/users/%s/messages" // openid

)

type Ticket struct {
	AppID  uint64
	Secret string
	Token  string
}

//   "d": {
//     "version": 1,
//     "session_id": "082ee18c-0be3-491b-9d8b-fbd95c51673a",
//     "user": {
//       "id": "6158788878435714165",
//       "username": "群pro测试机器人",
//       "bot": true
//     },
//     "shard": [0, 0]
//   }

type ReadyEvent struct {
	Version   int
	SessionId string
	User      struct {
		ID       string
		Username string
		Bot      bool
	}
	Shard []int
}

// type {
//   "name": "私密子频道",
//   "type": 0,
//   "position": 1,
//   "parent_id": "123456",
//   "owner_id": "0",
//   "sub_type": 0,
//   "private_type": 1
// }

type Channel struct {
	// 频道ID
	ID string `json:"id"`
	// 群ID
	GuildID string `json:"guild_id"`
	ChannelInfo
}

// ChannelValueObject 频道的值对象部分
type ChannelInfo struct {
	Name     string `json:"name,omitempty"`
	Type     int    `json:"type,omitempty"`
	SubType  int    `json:"sub_type,omitempty"`
	Position int64  `json:"position,omitempty"`
	ParentID string `json:"parent_id,omitempty"`

	// OwnerID         string   `json:"owner_id,omitempty"`
	// PrivateType     int      `json:"private_type,omitempty"`
	// PrivateUserIDs  []string `json:"private_user_ids,omitempty"`
	// SpeakPermission int      `json:"speak_permission,omitempty"`
	// ApplicationID   string   `json:"application_id,omitempty"`
	// Permissions     string   `json:"permissions,omitempty"`
	// OpUserID        string   `json:"op_user_id,omitempty"`
}

type EventHandler func(WsMessage)

type API struct {
	Target string
	Client *http.Client

	Ticket Ticket

	Intents  int
	Handlers map[string]EventHandler
}

func BotToken(ticket Ticket) string {
	token := fmt.Sprintf("Bot %d.%s", ticket.AppID, ticket.Token)
	return token
}

func (a *API) newRequest(method, api string, body io.Reader) (*http.Request, error) {
	gateway := a.Target
	if len(gateway) == 0 {
		gateway = SandboxSgroupTarget
	}
	url := api
	if !strings.HasPrefix(api, "http") {
		url = gateway + api
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", BotToken(a.Ticket))

	return req, nil
}

func (a *API) newRequestWithObject(method, api string, obj interface{}) (*http.Request, error) {
	var body io.Reader
	if obj != nil {
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		fmt.Println("newRequestWithObject", api, string(data))
		body = bytes.NewReader(data)
	}

	return a.newRequest(method, api, body)
}

func (a *API) doRequest(req *http.Request, result interface{}) error {
	cli := a.Client
	if cli == nil {
		cli = http.DefaultClient
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}

	var b = make([]byte, 0, resp.ContentLength)

	b, err = ReadAll(b, resp.Body)
	if err != nil {
		return fmt.Errorf("readAll: %w", err)
	}

	fmt.Println("doRequest", resp.StatusCode, string(b))

	if err := json.Unmarshal(b, result); err != nil {
		return err
	}

	return nil
}

func (a *API) doSimpleRequest(method, api string, request, response interface{}) error {

	req, err := a.newRequestWithObject(method, api, request)
	if err != nil {
		return err
	}

	if err := a.doRequest(req, response); err != nil {
		return err
	}

	return nil
}

func (a *API) CreateChannel(guildID string, request ChannelInfo) (*Channel, error) {
	method := http.MethodPost
	api := fmt.Sprintf(CreateChannelAPI, guildID)
	var result Channel
	if err := a.doSimpleRequest(method, api, &request, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type GuildListRequest struct {
	Before string `json:"before"`
	After  string `json:"after"`
	Limit  int    `json:"limit"`
}

type Guild struct {
	ID          string `json:"id"`           //string	频道ID
	Name        string `json:"name"`         //string	频道名称
	Icon        string `json:"icon"`         //string	频道头像地址
	OwnerID     string `json:"owner_id"`     //string	创建人用户ID
	Owner       bool   `json:"owner"`        //bool	当前人是否是创建人
	MemberCount int    `json:"member_count"` //int	成员数
	MaxMembers  int    `json:"max_members"`  //int	最大成员数
	Desc        string `json:"description"`  //string	描述
	JoinedAt    string `json:"joined_at"`    //string	加入时间
}

func (a *API) GetGuildList(request GuildListRequest) ([]Guild, error) {
	method := http.MethodGet
	api := GetGuildListAPI
	var result []Guild
	if err := a.doSimpleRequest(method, api, &request, &result); err != nil {
		return nil, err
	}

	return result, nil
}

type GatewayInfo struct {
	URL               string `json:"url"`
	Shards            int    `json:"shards"`
	SessionStartLimit struct {
		Total          int   `json:"total"`
		Remaining      int   `json:"remaining"`
		ResetAfter     int64 `json:"reset_after"`
		MaxConcurrency int   `json:"max_concurrency"`
	} `json:"session_start_limit"`
	// {
	//   "wss://api.sgroup.qq.com/websocket/",
	//   "shards": 9,
	//   "session_start_limit": {
	//     "total": 1000,
	//     "remaining": 999,
	//     "reset_after": 14400000,
	//     "max_concurrency": 1
	//   }
}

func (a *API) Gateway() (*GatewayInfo, error) {
	method := http.MethodGet
	api := GatewayAPI
	var result GatewayInfo
	if err := a.doSimpleRequest(method, api, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

const (
	MsgTypeText     = 0
	MsgTypeMarkdown = 2
	MsgTypeArk      = 3
	MsgTypeEmbed    = 4
)

type GroupMessageRequest struct {
	Content string `json:"content"`
	MsgType int    `json:"msg_type"`
	MsgID   string `json:"msg_id"`
	EventID string `json:"event_id,omitempty"`
	MsgSeq  int    `json:"msg_seq,omitempty"`
}

type GroupMessageResposne struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
}

func (a *API) CreateGroupMessage(groupOpenID string, msg GroupMessageRequest) error {
	method := http.MethodPost
	api := fmt.Sprintf(CreateGroupMessageAPI, groupOpenID)
	var result GroupMessageResposne
	if err := a.doSimpleRequest(method, api, &msg, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("code: %d, msg: %s", result.Code, result.Message)
	}

	return nil
}

func (a *API) CreateUserMessage(groupOpenID string, msg GroupMessageRequest) error {
	method := http.MethodPost
	api := fmt.Sprintf(CreateUserMessageAPI, groupOpenID)
	var result GroupMessageResposne
	if err := a.doSimpleRequest(method, api, &msg, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("code: %d, msg: %s", result.Code, result.Message)
	}

	return nil
}
