package sgroupbot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	OpDispatch        = 0  // Dispatch	Receive	服务端进行消息推送
	OpHeartbeat       = 1  // Heartbeat	Send/Receive	客户端或服务端发送心跳
	OpIdentify        = 2  // Identify	Send	客户端发送鉴权
	OpResume          = 6  // Resume	Send	客户端恢复连接
	OpReconnect       = 7  // Reconnect	Receive	服务端通知客户端重新连接
	OpInvalid         = 9  // Invalid Session	Receive	当identify或resume的时候，如果参数有错，服务端会返回该消息
	OpHello           = 10 // Hello	Receive	当客户端与网关建立ws连接之后，网关下发的第一条消息
	OpHeartbeatAck    = 11 // Heartbeat ACK	Receive/Reply	当发送心跳成功之后，就会收到该消息
	OpHTTPCallbackAck = 12 // HTTP Callback ACK	Reply	仅用于 http 回调模式的回包，代表机器人收到了平台推送的数据
)

// 事件类型
const (
	EventGuildCreate           string = "GUILD_CREATE"
	EventGuildUpdate           string = "GUILD_UPDATE"
	EventGuildDelete           string = "GUILD_DELETE"
	EventChannelCreate         string = "CHANNEL_CREATE"
	EventChannelUpdate         string = "CHANNEL_UPDATE"
	EventChannelDelete         string = "CHANNEL_DELETE"
	EventGuildMemberAdd        string = "GUILD_MEMBER_ADD"
	EventGuildMemberUpdate     string = "GUILD_MEMBER_UPDATE"
	EventGuildMemberRemove     string = "GUILD_MEMBER_REMOVE"
	EventMessageCreate         string = "MESSAGE_CREATE"
	EventMessageReactionAdd    string = "MESSAGE_REACTION_ADD"
	EventMessageReactionRemove string = "MESSAGE_REACTION_REMOVE"
	EventAtMessageCreate       string = "AT_MESSAGE_CREATE"
	EventPublicMessageDelete   string = "PUBLIC_MESSAGE_DELETE"
	EventDirectMessageCreate   string = "DIRECT_MESSAGE_CREATE"
	EventDirectMessageDelete   string = "DIRECT_MESSAGE_DELETE"
	EventAudioStart            string = "AUDIO_START"
	EventAudioFinish           string = "AUDIO_FINISH"
	EventAudioOnMic            string = "AUDIO_ON_MIC"
	EventAudioOffMic           string = "AUDIO_OFF_MIC"
	EventMessageAuditPass      string = "MESSAGE_AUDIT_PASS"
	EventMessageAuditReject    string = "MESSAGE_AUDIT_REJECT"
	EventMessageDelete         string = "MESSAGE_DELETE"
	EventForumThreadCreate     string = "FORUM_THREAD_CREATE"
	EventForumThreadUpdate     string = "FORUM_THREAD_UPDATE"
	EventForumThreadDelete     string = "FORUM_THREAD_DELETE"
	EventForumPostCreate       string = "FORUM_POST_CREATE"
	EventForumPostDelete       string = "FORUM_POST_DELETE"
	EventForumReplyCreate      string = "FORUM_REPLY_CREATE"
	EventForumReplyDelete      string = "FORUM_REPLY_DELETE"
	EventForumAuditResult      string = "FORUM_PUBLISH_AUDIT_RESULT"
	EventInteractionCreate     string = "INTERACTION_CREATE"
	EventGroupAtMessageCreate         = "GROUP_AT_MESSAGE_CREATE"
	EventC2CMessageCreate             = "C2C_MESSAGE_CREATE"
)

//	{
//	    "author": {
//	        "id": "4381EC917E8332933D6A2B7893C28ACB",
//	        "member_openid": "4381EC917E8332933D6A2B7893C28ACB"
//	    },
//	    "content": " aaa",
//	    "group_id": "0DE2782CA6DD4FB3B29730FC6F7C26AC",
//	    "group_openid": "0DE2782CA6DD4FB3B29730FC6F7C26AC",
//	    "id": "ROBOT1.0_FKati8czlWPoNcL7gK9wc.RS6sD5FukvrWRVXHhnwFcp5f71hM9ano1bK01zCwX36lZAu5CzMPWbxt85RoWg58tG81ovPjw88HwjHppK6Gc!",
//	    "timestamp": "12024-09-04T13:12:43+08:00"
//	}
type GroupMessage struct {
	Author struct {
		ID           string `json:"id"`
		MemberOpenID string `json:"member_openid"`
		UserOpenID   string `json:"user_openid"`
	} `json:"author"`
	Content     string `json:"content"`
	GroupID     string `json:"group_id"`
	GroupOpenID string `json:"group_openid"`
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`
}

const (

	// 	GUILDS (1 << 0)
	//   - GUILD_CREATE           // 当机器人加入新guild时
	//   - GUILD_UPDATE           // 当guild资料发生变更时
	//   - GUILD_DELETE           // 当机器人退出guild时
	//   - CHANNEL_CREATE         // 当channel被创建时
	//   - CHANNEL_UPDATE         // 当channel被更新时
	//   - CHANNEL_DELETE         // 当channel被删除时
	IntentGuilds = 1 << 0

	// GUILD_MEMBERS (1 << 1)
	//   - GUILD_MEMBER_ADD       // 当成员加入时
	//   - GUILD_MEMBER_UPDATE    // 当成员资料变更时
	//   - GUILD_MEMBER_REMOVE    // 当成员被移除时
	IntentGuildMembers = 1 << 1

	// GUILD_MESSAGES (1 << 9)    // 消息事件，仅 *私域* 机器人能够设置此 intents。
	//   - MESSAGE_CREATE         // 发送消息事件，代表频道内的全部消息，而不只是 at 机器人的消息。内容与 AT_MESSAGE_CREATE 相同
	//   - MESSAGE_DELETE         // 删除（撤回）消息事件
	IntentGuildMessages = 1 << 9

	// GUILD_MESSAGE_REACTIONS (1 << 10)
	//   - MESSAGE_REACTION_ADD    // 为消息添加表情表态
	//   - MESSAGE_REACTION_REMOVE // 为消息删除表情表态
	IntentGuildMessageReactions = 1 << 10

	// DIRECT_MESSAGE (1 << 12)
	//   - DIRECT_MESSAGE_CREATE   // 当收到用户发给机器人的私信消息时
	//   - DIRECT_MESSAGE_DELETE   // 删除（撤回）消息事件
	IntentDriectMessage = 1 << 12

	// GROUP_AND_C2C_EVENT (1 << 25)
	//   - C2C_MESSAGE_CREATE      // 用户单聊发消息给机器人时候
	//   - FRIEND_ADD              // 用户添加使用机器人
	//   - FRIEND_DEL              // 用户删除机器人
	//   - C2C_MSG_REJECT          // 用户在机器人资料卡手动关闭"主动消息"推送
	//   - C2C_MSG_RECEIVE         // 用户在机器人资料卡手动开启"主动消息"推送开关
	//   - GROUP_AT_MESSAGE_CREATE // 用户在群里@机器人时收到的消息
	//   - GROUP_ADD_ROBOT         // 机器人被添加到群聊
	//   - GROUP_DEL_ROBOT         // 机器人被移出群聊
	//   - GROUP_MSG_REJECT        // 群管理员主动在机器人资料页操作关闭通知
	//   - GROUP_MSG_RECEIVE       // 群管理员主动在机器人资料页操作开启通知
	IntentGroupAndC2CEvent = 1 << 25

	// INTERACTION (1 << 26)
	//   - INTERACTION_CREATE     // 互动事件创建时
	IntentInteraction = 1 << 26

	// MESSAGE_AUDIT (1 << 27)
	// - MESSAGE_AUDIT_PASS     // 消息审核通过
	// - MESSAGE_AUDIT_REJECT   // 消息审核不通过
	IntentMessageAudit = 1 << 28

	// FORUMS_EVENT (1 << 28)  // 论坛事件，仅 *私域* 机器人能够设置此 intents。
	//   - FORUM_THREAD_CREATE     // 当用户创建主题时
	//   - FORUM_THREAD_UPDATE     // 当用户更新主题时
	//   - FORUM_THREAD_DELETE     // 当用户删除主题时
	//   - FORUM_POST_CREATE       // 当用户创建帖子时
	//   - FORUM_POST_DELETE       // 当用户删除帖子时
	//   - FORUM_REPLY_CREATE      // 当用户回复评论时
	//   - FORUM_REPLY_DELETE      // 当用户回复评论时
	//   - FORUM_PUBLISH_AUDIT_RESULT      // 当用户发表审核通过时
	IntentForumsEvent = 1 << 28

	// AUDIO_ACTION (1 << 29)
	//   - AUDIO_START             // 音频开始播放时
	//   - AUDIO_FINISH            // 音频播放结束时
	//   - AUDIO_ON_MIC            // 上麦时
	//   - AUDIO_OFF_MIC           // 下麦时
	IntentAudioAction = 1 << 29

	// PUBLIC_GUILD_MESSAGES (1 << 30) // 消息事件，此为公域的消息事件
	//   - AT_MESSAGE_CREATE       // 当收到@机器人的消息时
	//   - PUBLIC_MESSAGE_DELETE   // 当频道的消息被删除时

	IntentPublicGuildMessages = 1 << 30
)

type WsMessage struct {
	MessageHeader
	Data json.RawMessage `json:"d"`
}

type MessageHeader struct {
	Op   int    `json:"op"`
	Seq  uint32 `json:"s"`
	Type string `json:"t"`
	// Data     json.RawMessage
}

type HeartbeatMessage struct {
	MessageHeader
	Seq uint32 `json:"d,omitempty"`
}

type IdentifyData struct {
	Token   string `json:"token"`
	Intents int    `json:"intents"`
	Shard   [2]int `json:"shard"`
	// Propertie
}

type IdentifyMessage struct {
	MessageHeader
	Data IdentifyData `json:"d"`
}

func (a *API) StartWs(ctx context.Context) error {
	gw, err := a.Gateway()
	if err != nil {
		return err
	}

	url := gw.URL

	// token := fmt.Sprintf("Bot %d.%s", a.Ticket.AppID, a.Ticket.Token)
	// header := http.Header{}
	// header.Set("Authorization", token)
	// d := websocket.Dialer{}
	// ws, _, err := d.Dial(url, nil)
	// if err != nil {
	// 	return err
	// }

	// var sessions = make(chan Session)
	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 	case <-sessions:

	// 	}
	// }
	return a.runSession(ctx, url, 0, 1, a.Intents)
}

type Session struct {
	Seq  uint32
	Conn *websocket.Conn
}

func (a *API) runSession(ctx context.Context, gateway string, shardIndex, shardTotal int, intent int) error {
	// 1. connect
	d := websocket.Dialer{}
	ws, _, err := d.Dial(gateway, nil)
	if err != nil {
		return err
	}

	// 2. identify
	var identify IdentifyMessage
	identify.Op = OpIdentify
	identify.Data.Token = BotToken(a.Ticket)
	identify.Data.Intents = intent
	// identify.Data.Intents = math.MaxInt32
	identify.Data.Shard = [2]int{shardIndex, shardTotal}

	if err := ws.WriteJSON(&identify); err != nil {
		return err
	}

	var session = Session{
		Conn: ws,
	}

	go a.heartBeat(ctx, &session)

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return err
		}

		fmt.Println("received", string(data))
		var msg WsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}

		if msg.Seq > 0 {
			atomic.StoreUint32(&session.Seq, msg.Seq)
		}

		a.dispatch(msg)
		// switch msg.Op {
		// case OpDispatch:
		// default:
		// 	// 忽略其它类型的消息
		// }
	}

	return nil
}

func (a *API) heartBeat(ctx context.Context, session *Session) error {
	var heartBeat HeartbeatMessage
	var ws = session.Conn
	if err := ws.WriteJSON(&heartBeat); err != nil {
		return err
	}
	t := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			//
		case <-t.C:
			heartBeat.Seq = atomic.LoadUint32(&session.Seq)
			if err := ws.WriteJSON(&heartBeat); err != nil {
				fmt.Println("heartbeat", err)
				return err
			}
		}
	}
}

func (a *API) dispatch(msg WsMessage) {
	fmt.Println(len(a.Handlers), msg.Op, msg.Seq, msg.Type, string(msg.Data))
	if len(a.Handlers) == 0 {
		return
	}

	if h, ok := a.Handlers[msg.Type]; ok {
		h(msg)
	}
}
