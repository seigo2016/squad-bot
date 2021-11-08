package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var ctx = context.Background()
var rSquadClient *redis.Client
var rConfigClient *redis.Client

type SquadInfo struct {
	VoiceChannelID string
	TextChannelID  string
	MessageID      string
	Name           string
}

type ServerConfig struct {
	VoiceChannelID string
	ParentID       string
	NameOption     []string
}

var voiceStateList []*discordgo.VoiceState
var emojis = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣"}
var (
	token  string
	dbhost string
)

func main() {
	err := godotenv.Load(".env")
	dbhost = os.Getenv("dbhost")
	token = os.Getenv("token")
	stopBot := make(chan bool)
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Failed to Start bot ", err)
	}
	rSquadClient = redis.NewClient(&redis.Options{
		Addr:     dbhost,
		Password: "",
		DB:       0,
	})
	rConfigClient = redis.NewClient(&redis.Options{
		Addr:     dbhost,
		Password: "",
		DB:       1,
	})
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	discord.AddHandler(ready)
	discord.AddHandler(voiceStateUpdate)
	discord.AddHandler(guildCreate)
	discord.AddHandler(onMessageReactionAdd)
	err = discord.Open()
	if err != nil {
		fmt.Println("Failed to Connect to Discord ", err)
	}
	defer discord.Close()
	fmt.Println("Listening...")
	<-stopBot
	return
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	for _, v := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			fmt.Println("Failed to Create Application Command ", err)
		}
	}
}

func guildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	g := e.Guild
	voiceStateList = append(voiceStateList, g.VoiceStates...)
}

func createSquad(s *discordgo.Session, vs *discordgo.VoiceStateUpdate, catid string) {
	vChData := discordgo.GuildChannelCreateData{
		Name:      "squad",
		Type:      discordgo.ChannelTypeGuildVoice,
		UserLimit: 0,
		ParentID:  catid,
	}
	tChData := discordgo.GuildChannelCreateData{
		Name:      "squad",
		Type:      discordgo.ChannelTypeGuildText,
		UserLimit: 0,
		ParentID:  catid,
	}
	tst, err := s.GuildChannelCreateComplex(vs.GuildID, tChData)
	if err != nil {
		fmt.Printf("Failed to Create Squad Text Channel (GuildID:%s) %s\n", vs.GuildID, err)
	}
	vst, err := s.GuildChannelCreateComplex(vs.GuildID, vChData)
	if err != nil {
		fmt.Printf("Failed to Create Squad Voice Channel (GuildID:%s) %s\n", vs.GuildID, err)
	}
	temp := "<@!%s> \n 小隊が編成されました。\n名前を選択してください\n"
	val, err := rConfigClient.Get(ctx, vs.GuildID).Bytes()
	n := ""
	if err == redis.Nil || fmt.Sprintf("%s", err) == "redis: nil" {
		n = "名前は登録されていません"
	} else if err != nil {
		fmt.Println("redis.Client.Get Error ", err)
		n = "データベースError"
	} else {
		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		msg := ""
		fmt.Println()
		if len(deserialized.NameOption) == 0 {
			n = "名前は登録されていません"
		} else {
			for i, v := range deserialized.NameOption {
				msg = fmt.Sprintf("%s% s:  %s\n", msg, emojis[i], v)
			}
			n = msg
		}
	}
	squadMsg := fmt.Sprintf(temp, vs.UserID)
	msg, err := s.ChannelMessageSend(tst.ID, squadMsg+"\n"+n)
	if err != nil {
		fmt.Println(err)
	}
	cval, err := rConfigClient.Get(ctx, vs.GuildID).Bytes()
	if err == redis.Nil || fmt.Sprintf("%s", err) == "redis: nil" {
		return
	} else if err != nil {
		fmt.Println("redis.Client.Get Error ", err)
		return
	}
	dc := new(ServerConfig)
	json.Unmarshal(cval, dc)
	for i, _ := range dc.NameOption {
		s.MessageReactionAdd(msg.ChannelID, msg.ID, emojis[i])
	}
	err = s.GuildMemberMove(vs.GuildID, vs.UserID, &vst.ID)
	if err != nil {
		fmt.Println("Failed to Move a Member", err)
	}
	squad := &SquadInfo{
		VoiceChannelID: vst.ID,
		TextChannelID:  tst.ID,
		MessageID:      msg.ID,
		Name:           "squad",
	}
	serialized, _ := json.Marshal(squad)
	key := vs.GuildID + "/" + vst.ID
	err = rSquadClient.Set(ctx, key, string(serialized), 0).Err()
	if err != nil {
		fmt.Println("redis.Client.Set Error ", err)
	}
}

func editChannelName(key, name string, deserialized *SquadInfo, rSquadClient *redis.Client) {
	squad := &SquadInfo{
		VoiceChannelID: deserialized.VoiceChannelID,
		TextChannelID:  deserialized.TextChannelID,
		MessageID:      deserialized.MessageID,
		Name:           name,
	}
	serialized, _ := json.Marshal(squad)
	err := rSquadClient.Set(ctx, key, string(serialized), 0).Err()
	if err != nil {
		fmt.Println("redis.Client.Set Error ", err)
	}
}

func onMessageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	messageID := r.MessageID
	sq, _ := rSquadClient.Keys(ctx, r.GuildID+"/*").Result()
	for _, k := range sq {
		val, err := rSquadClient.Get(ctx, k).Bytes()
		cval, err := rConfigClient.Get(ctx, r.GuildID).Bytes()
		if err == redis.Nil || fmt.Sprintf("%s", err) == "redis: nil" {
			return
		} else if err != nil {
			fmt.Println("redis.Client.Get Error ", err)
			continue
		}
		ds := new(SquadInfo)
		dc := new(ServerConfig)
		json.Unmarshal(val, ds)
		json.Unmarshal(cval, dc)
		if ds.MessageID == messageID {
			for i, v := range emojis {
				if v != r.Emoji.Name {
					continue
				}
				name := dc.NameOption[i]
				editChannelName(k, name, ds, rSquadClient)
				s.ChannelEdit(ds.TextChannelID, name)
				s.ChannelEdit(ds.VoiceChannelID, name)
			}
		}
	}
}

func voiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// ロビーに入室→Squad作成
	val, err := rConfigClient.Get(ctx, vs.GuildID).Bytes()
	if err == redis.Nil || fmt.Sprintf("%s", err) == "redis: nil" {
		fmt.Println("redis.Client.Get Error ", err)
	} else if err != nil {
		fmt.Println("redis.Client.Get Error ", err)
	} else {
		deserialized := new(ServerConfig)
		json.Unmarshal(val, deserialized)
		if vs.ChannelID == deserialized.VoiceChannelID {
			createSquad(s, vs, deserialized.ParentID)
		}
	}
	// voiceState更新
	// 前の状態を削除
	for i, v := range voiceStateList {
		if vs.BeforeUpdate != nil && *v == *vs.BeforeUpdate {
			voiceStateList[i] = voiceStateList[len(voiceStateList)-1]
			voiceStateList = voiceStateList[:len(voiceStateList)-1]
		}
	}
	if vs.ChannelID != "" {
		_, err := rSquadClient.Get(ctx, vs.GuildID+"/"+vs.ChannelID).Bytes()
		if err != redis.Nil {
			voiceStateList = append(voiceStateList, vs.VoiceState)
		} else if err != nil {
			fmt.Println("redis.Client.Get Error ", err)
		}
	} else if vs.ChannelID == "" {
		// 該当チャンネルのメンバーが0なら削除
		count := 0
		for _, v := range voiceStateList {
			if vs.BeforeUpdate != nil && v.ChannelID == vs.BeforeUpdate.ChannelID {
				count++
				break
			}
		}
		if count == 0 {
			// 削除
			val, err := rSquadClient.Get(ctx, vs.GuildID+"/"+vs.BeforeUpdate.ChannelID).Bytes()
			if err == redis.Nil || fmt.Sprintf("%s", err) == "redis: nil" {
				fmt.Println("redis.Client.Get Error ", err)
				return
			} else if err != nil {
				fmt.Println("redis Error ", err)
				return
			}
			deserialized := new(SquadInfo)
			json.Unmarshal(val, deserialized)
			s.ChannelDelete(deserialized.VoiceChannelID)
			s.ChannelDelete(deserialized.TextChannelID)
			rSquadClient.Del(ctx, vs.GuildID+"/"+vs.BeforeUpdate.ChannelID)
		}
	}
}
