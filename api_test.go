package sgroupbot_test

import (
	"sgroupbot"
	"testing"
)

var ticket = sgroupbot.Ticket{
	AppID: 00,
	Token: "xxxx",
}

var api = sgroupbot.API{
	Target: sgroupbot.SandboxSgroupTarget,
	Ticket: ticket,
}

func TestCreateChannel(t *testing.T) {

	var listReq = sgroupbot.GuildListRequest{
		Limit: 100,
	}
	list, err := api.GetGuildList(listReq)
	if err != nil {
		t.Log(err)
		return
	}

	t.Log(list)

	guildID := "17581271750430039607"

	channelInfo := sgroupbot.ChannelInfo{
		Name:     "机器人测试",
		Position: 1,
	}

	channel, err := api.CreateChannel(guildID, channelInfo)
	if err != nil {
		t.Log(err)
		return
	}

	t.Log(channel)

}

func TestWs(t *testing.T) {
}
