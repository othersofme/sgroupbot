package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sgroupbot"
)

func main() {
	var ticket = sgroupbot.Ticket{
		AppID: 00,
		Token: "xxx",
	}

	intents := sgroupbot.IntentGroupAndC2CEvent | sgroupbot.IntentPublicGuildMessages
	var api = sgroupbot.API{
		Target:   sgroupbot.SandboxSgroupTarget,
		Ticket:   ticket,
		Intents:  intents,
		Handlers: make(map[string]sgroupbot.EventHandler),
	}

	var s = ApiServer{api: &api}

	api.Handlers[sgroupbot.EventGroupAtMessageCreate] = s.HandleMessage
	api.Handlers[sgroupbot.EventC2CMessageCreate] = s.HandleMessage

	err := api.StartWs(context.Background())
	if err != nil {
		fmt.Println("startWs", err)
		return
	}
}

type ApiServer struct {
	api *sgroupbot.API
}

// Idioms Solitaire
func (s *ApiServer) HandleMessage(msg sgroupbot.WsMessage) {
	fmt.Println(msg.Type, string(msg.Data))
	if msg.Type == sgroupbot.EventGroupAtMessageCreate || msg.Type == sgroupbot.EventC2CMessageCreate {
		var gm sgroupbot.GroupMessage
		if err := json.Unmarshal(msg.Data, &gm); err != nil {
			fmt.Println(err)
			return
		}

		var resp sgroupbot.GroupMessageRequest
		resp.Content = gm.Content
		resp.MsgType = sgroupbot.MsgTypeText
		resp.MsgID = gm.ID
		// resp.EventID = msg.Type
		// resp.EventID = msg.Type
		// resp.MsgSeq = 12345
		if msg.Type == sgroupbot.EventGroupAtMessageCreate {

			if err := s.api.CreateGroupMessage(gm.GroupOpenID, resp); err != nil {
				fmt.Println("sendMsg", err)
			}
		} else {

			if err := s.api.CreateUserMessage(gm.Author.UserOpenID, resp); err != nil {
				fmt.Println("sendMsg", err)
			}
		}

	}
}
