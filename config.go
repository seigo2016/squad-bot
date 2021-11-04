package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "set-lobby",
			Description: "set-lobby",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel-option",
				Description: "Channel option",
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildVoice,
				},
				Required: true,
			},
			},
		},
		{
			Name:        "del-lobby",
			Description: "del-lobby",
		},
		{
			Name:        "show-messages",
			Description: "show-messages",
		},
		{
			Name:        "status",
			Description: "status",
		},
		{
			Name:        "add-message",
			Description: "add-message",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "string-option",
				Description: "string option",
				Required:    true,
			},
			},
		},
		{
			Name:        "del-message",
			Description: "del-message",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "integer-option",
				Description: "Integer option",
				Required:    true,
			},
			},
		},
	}
	commandHandlers = map[string]func(*discordgo.Session, *discordgo.InteractionCreate){
		"set-lobby": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				addLobbyChannel(s, i)
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
		"del-lobby": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				delLobbyChannel(s, i)
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
		"show-messages": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				showOptionMessages(s, i)
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
		"add-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				addOptionMessage(s, i)
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
		"del-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				delOptionMessage(s, i)
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
		"status": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			f := checkRole(s, i)
			if f {
				text := getStatus(s, i)
				embed := &discordgo.MessageEmbed{
					Author:      &discordgo.MessageEmbedAuthor{},
					Color:       0x0000ff,
					Description: text,
					Title:       "現在の設定",
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{embed},
					},
				})
			} else {
				sendTextMessage(s, i, "権限が有りません", "error")
			}
		},
	}
)

func checkRole(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	for _, v := range i.Member.Roles {
		r, _ := s.State.Role(i.GuildID, v)
		if r.Name == "Squad" {
			return true
		}
	}
	return false
}

func sendTextMessage(s *discordgo.Session, i *discordgo.InteractionCreate, text string, t string) {
	color := 0xeeeeee
	if t == "error" {
		color = 0xff0000
	} else if t == "notice" {
		color = 0x00ff00
	}
	embed := &discordgo.MessageEmbed{
		Author:      &discordgo.MessageEmbedAuthor{},
		Color:       color,
		Description: text,
		Title:       "Result",
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func addLobbyChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	val, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	key := i.GuildID
	chid := i.ApplicationCommandData().Options[0].Value.(string)
	ch, _ := s.Channel(chid)
	if err == redis.Nil {
		sc := &ServerConfig{
			VoiceChannelID: chid,
			ParentID:       ch.ParentID,
			NameOption:     []string{},
		}
		serialized, _ := json.Marshal(sc)
		rConfigClient.Set(ctx, key, serialized, 0)
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		sendTextMessage(s, i, "Error", "error")
		return
	} else {
		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		sc := &ServerConfig{
			VoiceChannelID: i.ApplicationCommandData().Options[0].Value.(string),
			ParentID:       ch.ParentID,
			NameOption:     deserialized.NameOption,
		}
		serialized, _ := json.Marshal(sc)
		rConfigClient.Set(ctx, key, serialized, 0)
	}
	sendTextMessage(s, i, "登録しました", "notice")
}

func delLobbyChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	key := i.GuildID
	_, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	if err == redis.Nil {
		sendTextMessage(s, i, "登録されていません", "error")
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		sendTextMessage(s, i, "Error", "error")
	} else {
		rConfigClient.Del(ctx, key)
		sendTextMessage(s, i, "登録を解除しました", "notice")
	}

}

func addOptionMessage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	val, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	key := i.GuildID
	m := i.ApplicationCommandData().Options[0].Value.(string)
	if err == redis.Nil {
		n := []string{}
		n = append(n, m)
		sc := &ServerConfig{
			VoiceChannelID: "",
			ParentID:       "",
			NameOption:     n,
		}
		serialized, _ := json.Marshal(sc)
		rConfigClient.Set(ctx, key, serialized, 0)
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		sendTextMessage(s, i, "Error", "error")
	} else {
		f, err := regexp.MatchString("^[0-9a-zA-Z]+$", m)
		if err != nil {
			fmt.Println(err)
		}
		if f {
			deserialized := new(ServerConfig)
			json.Unmarshal(val, deserialized)
			n := append(deserialized.NameOption, m)
			sc := &ServerConfig{
				VoiceChannelID: deserialized.VoiceChannelID,
				ParentID:       deserialized.ParentID,
				NameOption:     n,
			}
			serialized, _ := json.Marshal(sc)
			rConfigClient.Set(ctx, key, serialized, 0)
			sendTextMessage(s, i, "登録しました", "notice")
		} else {
			sendTextMessage(s, i, "使用可能な文字は英数字のみです", "error")
		}
	}
}

func delOptionMessage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	val, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	key := i.GuildID
	id := int(i.ApplicationCommandData().Options[0].Value.(float64))
	if err == redis.Nil {
		sendTextMessage(s, i, "登録されていません", "error")
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		sendTextMessage(s, i, "Error", "error")
		return
	} else {
		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		n := deserialized.NameOption
		for i, _ := range n {
			if i == id-1 {
				n[i] = n[len(n)-1]
				n = n[:len(n)-1]
			}
		}
		sc := &ServerConfig{
			VoiceChannelID: deserialized.VoiceChannelID,
			ParentID:       deserialized.ParentID,
			NameOption:     n,
		}
		serialized, _ := json.Marshal(sc)
		rConfigClient.Set(ctx, key, serialized, 0)
		sendTextMessage(s, i, "削除しました", "notice")
	}
}

func showOptionMessages(s *discordgo.Session, i *discordgo.InteractionCreate) {
	val, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	if err == redis.Nil {
		sendTextMessage(s, i, "登録されていません", "error")
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		sendTextMessage(s, i, "Error", "error")
	} else {

		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		msg := ""
		if len(deserialized.NameOption) == 0 {
			sendTextMessage(s, i, "登録されていません", "error")
		} else {
			for i, v := range deserialized.NameOption {
				msg = fmt.Sprintf("%s% s:  %s\n", msg, emojis[i], v)
			}
			sendTextMessage(s, i, msg, "notice")
		}
	}
}

func getStatus(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	msg := ""
	val, err := rConfigClient.Get(ctx, i.GuildID).Bytes()
	if err == redis.Nil {
		msg = "登録されていません"
	} else if err != nil {
		fmt.Println("redis.Client.Get Error:", err)
		msg = "Error"
	} else {
		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		lobby := "__ロビーチャンネル__\n"
		if deserialized.VoiceChannelID == "" {
			lobby = "登録されていません\n\n"
		} else {
			ch, _ := s.Channel(deserialized.VoiceChannelID)
			lobby += fmt.Sprintf("%s\n\n", ch.Mention())
		}
		opt := "__メッセージオプション__\n"
		if len(deserialized.NameOption) == 0 {
			opt = "登録されていません\n"
		} else {
			for i, v := range deserialized.NameOption {
				opt += fmt.Sprintf("%s:  %s\n", emojis[i], v)
			}

		}
		msg = lobby + opt
	}
	fmt.Println(msg)
	return msg
}
