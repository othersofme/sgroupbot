package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sgroupbot"
	"strconv"
	"strings"

	"github.com/panjf2000/ants/v2"
)

type ApiServer struct {
	api  *sgroupbot.API
	is   *IdiomsSolitaire
	pool *ants.PoolWithFunc
}

func NewApiServer(api *sgroupbot.API, is *IdiomsSolitaire) *ApiServer {
	s := &ApiServer{
		api: api,
		is:  is,
	}

	// 注册消息函数
	api.Intents = sgroupbot.IntentGroupAndC2CEvent |
		sgroupbot.IntentPublicGuildMessages | sgroupbot.IntentDriectMessage
	api.Handlers[sgroupbot.EventGroupAtMessageCreate] = s.HandleMessage
	api.Handlers[sgroupbot.EventC2CMessageCreate] = s.HandleMessage
	api.Handlers[sgroupbot.EventDirectMessageCreate] = s.HandleMessage
	api.Handlers[sgroupbot.EventAtMessageCreate] = s.HandleMessage

	s.pool, _ = ants.NewPoolWithFunc(128, func(i interface{}) {
		if msg, ok := i.(Message); ok {
			s.handleMessage(msg)
		}
	}, ants.WithNonblocking(true), ants.WithPreAlloc(false))

	return s
}

type MessageSender func(string, sgroupbot.CreateMessageRequest) error

func (s *ApiServer) handleMessage(msg Message) {
	var to string
	var sendMsg MessageSender
	var userID string
	switch msg.MsgType {
	case sgroupbot.EventGroupAtMessageCreate: // 群 at 消息
		to = msg.GroupOpenID
		userID = msg.Author.MemberOpenID
		sendMsg = s.api.CreateGroupMessage
	case sgroupbot.EventC2CMessageCreate: // 单聊
		to = msg.Author.UserOpenID
		userID = msg.Author.UserOpenID
		sendMsg = s.api.CreateUserMessage
	case sgroupbot.EventAtMessageCreate: // 频道 at 消息
		to = msg.ChannelID
		userID = msg.Author.Username
		sendMsg = s.api.CreateChannelMessage
	case sgroupbot.EventDirectMessageCreate: // 频道私聊
		to = msg.GuildID
		userID = msg.Author.Username
		sendMsg = s.api.CreateDirectMessage
	default:
		// fmt.Println("drop", msg.MsgType)
		// 不符合的消息类型，丢弃
		return
	}

	// 默认重复接收到的内容
	var rspMsg sgroupbot.CreateMessageRequest
	rspMsg.Content = msg.Content
	rspMsg.MsgType = sgroupbot.MsgTypeText
	rspMsg.MsgID = msg.ID

	// 处理成语接龙的逻辑
	key := to
	if msg.MsgType == sgroupbot.EventDirectMessageCreate {
		key = msg.Author.UserOpenID
	}

	content := strings.TrimSpace(msg.Content)
	switch content {
	case "成语接龙": // 进入情景
		if ss, loaded := s.is.SessionOrCreate(key); loaded {
			rspMsg.Content = "成语接龙正在进行中，想想这个成语怎么接，" + ss.Idiom()
		} else {
			log.Println("solitaire_create", key, ss.current, ss.lastRune)
			rspMsg.Content = "成语接龙开始了哦，想想这个成语怎么接，" + ss.Idiom()
		}
	case "退出": // 退出情景
		if _, exists := s.is.SessionAndDelete(key); exists {
			// 退出，输出结算
			// ss.Hits()
			rspMsg.Content = "成语接龙已结束"
			log.Println("solitaire_end", key, userID)
		}
	default: // 接龙
		if ss := s.is.Session(key); ss != nil {
			// 1. 接龙失败，返回提示
			// 2. 多次接龙失败，直接进入到下一轮，返回新的词
			// 3. 接龙成功，进入下一轮，返回新的词
			// 4. 接龙成功，接不下去了，直接结束，返回结算数据
			// 5. 接龙完成，会话结束，返回结算数据
			// 6. 会话已过期，或被其它人退出了
			ret, next := s.is.Solitaire(ss, content, userID)
			log.Println("solitaire_result", key, content, userID, ret, next)
			var settle bool
			switch ret {
			case SolitaireFailed:
				rspMsg.Content = "不是这个词哦，再想想"
			case SolitaireFailedToNext:
				rspMsg.Content = "还是不对哦，让我告诉你吧，" + next
			case SolitaireSucceed:
				rspMsg.Content = "你答对了，我接这个词，" + next
			case SolitaireEnd: // 输出结算
				rspMsg.Content = "你真厉害，我接不上来了，接龙结束"
				settle = true
			case SolitaireCompleted: // 输出结算
				rspMsg.Content = "你真厉害，全部完成了哦"
				settle = true
			case SolitaireFailComplete: // 输出结算
				rspMsg.Content = "接龙结束了，最后一个词可以接这个，" + next
				settle = true
			default:
			}

			if settle {
				if hits := ss.Hits(); len(hits) > 0 && hits[0].Count > 0 {
					var sb strings.Builder
					sb.WriteString(rspMsg.Content)
					sb.WriteByte('\n')
					sb.WriteString("接龙排行\n")
					for i := 0; i < len(hits) && i < 3 && hits[i].Count > 0; i++ {
						sb.WriteString(hits[i].Name)
						sb.WriteByte(' ')
						sb.WriteString(strconv.Itoa(hits[i].Count))
						sb.WriteByte('\n')
					}
					rspMsg.Content = sb.String()
				}
			}
		}
	}

	// 发送消息
	if err := sendMsg(to, rspMsg); err != nil {
		fmt.Println("sendMsg", err)
	}
}

type Message struct {
	MsgType string
	sgroupbot.Message
}

func (s *ApiServer) HandleMessage(wm sgroupbot.WsMessage) {
	// fmt.Println(wm.Type, string(wm.Data))

	// 解析消息内容
	var msg Message
	msg.MsgType = wm.Type
	if err := json.Unmarshal(wm.Data, &msg); err != nil {
		log.Println("unmarsahl", err)
		return
	}
	// 投递到线程池
	if err := s.pool.Invoke(msg); err != nil {
		// 线程池已满，同步执行
		s.handleMessage(msg)
	}
}

func (s *ApiServer) Start() error {
	if err := s.api.StartWs(context.Background()); err != nil {
		return err
	}
	return nil
}
