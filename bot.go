package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	URL              = "http://localhost:8065"
	TEAM_NAME        = "awc"
	CHANNEL_LOG_NAME = "system_update"

	BOT_TOKEN_SECRET = "h146hbzg6fnajnpn88jmtrwn5h"
	BOT_ID           = "5h4ifgkozpnuz89naqycuhcxmr"

	AWC_URL = "http://127.0.0.1:8041/index.php/"
)

var client *model.Client4
var webSocketClient *model.WebSocketClient

var botUser *model.User
var botTeam *model.Team
var debuggingChannel *model.Channel

func main() {
	client = model.NewAPIv4Client(URL)
	SetToken()
	GetBotUser()
	SetupGracefulShutdown()
	FindBotTeam()
	CreateBotDebuggingChannelIfNeeded()
	SendMsgToDebuggingChannel("bot "+botUser.Username+" has **started** running", "")

	webSocketClient, err := model.NewWebSocketClient4("ws://localhost:8065", client.AuthToken)
	if err != nil {
		println("We failed to connect to the web socket")
		PrintError(err)
	}

	webSocketClient.Listen()

	go func() {
		for {
			select {
			case resp := <-webSocketClient.EventChannel:
				HandleWebSocketResponse(resp)
			}
		}
	}()

	// You can block forever with
	select {}
}

func MakeSureServerIsRunning() {
	if props, resp := client.GetOldClientConfig(""); resp.Error != nil {
		println("There was a problem pinging the Mattermost server.  Are you sure it's running?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		println("Server detected and is running version " + props["Version"])
	}
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	HandleMsgFromDebuggingChannel(event)
}

func HandleMsgFromDebuggingChannel(event *model.WebSocketEvent) {
	// If this isn't the debugging channel then lets ingore it
	if channel, resp := client.GetChannel(event.Broadcast.ChannelId, ""); resp.Error != nil {
		println("There was a problem getting broadcast channel")
		PrintError(resp.Error)
	} else {
		if channel.Type != "D" {
			return
		}
	}

	// Lets only reponded to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	println("responding to msg")

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		dm, resp := client.CreateDirectChannel(BOT_ID, post.UserId)
		if resp.Error != nil {
			println("There was a problem getting direct message channel")
			PrintError(resp.Error)
		}

		// ignore my events
		if post.UserId == BOT_ID {
			return
		}

		// // if you see any word matching 'alive' then respond
		// if matched, _ := regexp.MatchString(`(?:^|\W)alive(?:$|\W)`, post.Message); matched {
		// 	SendMsgToDebuggingChannel("Yes I'm running", post.Id)
		// 	return
		// }

		// // if you see any word matching 'up' then respond
		// if matched, _ := regexp.MatchString(`(?:^|\W)up(?:$|\W)`, post.Message); matched {
		// 	SendMsgToDebuggingChannel("Yes I'm running", post.Id)
		// 	return
		// }

		// // if you see any word matching 'running' then respond
		// if matched, _ := regexp.MatchString(`(?:^|\W)running(?:$|\W)`, post.Message); matched {
		// 	SendMsgToDebuggingChannel("Yes I'm running", post.Id)
		// 	return
		// }

		// // if you see any word matching 'hello' then respond
		// if matched, _ := regexp.MatchString(`(?:^|\W)hello(?:$|\W)`, post.Message); matched {
		// 	SendMsgToDebuggingChannel("Yes I'm running", post.Id)
		// 	return
		// }

		if post.ParentId == "" {
			// if you see any word matching 'hello' then respond
			if matched, _ := regexp.MatchString(`(?:^|\W)leave day(?:$|\W)`, post.Message); matched {
				SendMsgToChannel(dm.Id, "You have 16 leave days left.", post.Id)
				return
			} else {
				SendMsgToChannel(dm.Id, "I did not understand you!", post.Id)
			}
		} else {
			form := url.Values{}
			form.Add("post_root", post.ParentId)
			form.Add("user_mm_id", post.UserId)
			form.Add("comment", post.Message)
			endpoint := "timesheet/broadcast_task_comment_except_user"
			//one-line post request/response...
			response, err := http.PostForm(AWC_URL+endpoint, form)

			//okay, moving on...
			if err != nil {
				//handle postform error
			}

			defer response.Body.Close()
			body, err := ioutil.ReadAll(response.Body)

			if err != nil {
				//handle read response error
			}

			fmt.Printf("%s\n", string(body))
			// if req, err := http.NewRequest("POST", AWC_URL+endpoint, strings.NewReader(form.Encode())); err != nil {
			// 	println("There was a problem sending post to AWC")
			// 	return
			// } else {
			// 	println(req.URL.Host)
			// 	println(req.URL.Path)
			// 	trace := &httptrace.ClientTrace{
			// 		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			// 			fmt.Printf("DNS Info: %+v\n", dnsInfo)
			// 		},
			// 		GotConn: func(connInfo httptrace.GotConnInfo) {
			// 			fmt.Printf("Got Conn: %+v\n", connInfo)
			// 		},
			// 	}
			// 	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
			// 	if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
			// 		log.Fatal(err)
			// 	}
			// }
			// SendMsgToChannel(dm.Id, "I did not understand you!", post.ParentId)
		}
	}

}

func FindBotTeam() {
	if team, resp := client.GetTeamByName(TEAM_NAME, ""); resp.Error != nil {
		println("We failed to get the initial load")
		println("or we do not appear to be a member of the team '" + TEAM_NAME + "'")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botTeam = team
	}
}

func CreateBotDebuggingChannelIfNeeded() {
	if rchannel, resp := client.GetChannelByName(CHANNEL_LOG_NAME, botTeam.Id, ""); resp.Error != nil {
		println("We failed to get the channels")
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		return
	}

	// Looks like we need to create the logging channel
	channel := &model.Channel{}
	channel.Name = CHANNEL_LOG_NAME
	channel.DisplayName = "Debugging For Sample Bot"
	channel.Purpose = "This is used as a test channel for logging bot debug messages"
	channel.Type = model.CHANNEL_OPEN
	channel.TeamId = botTeam.Id
	if rchannel, resp := client.CreateChannel(channel); resp.Error != nil {
		println("We failed to create the channel " + CHANNEL_LOG_NAME)
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		println("Looks like this might be the first run so we've created the channel " + CHANNEL_LOG_NAME)
	}
}

func SendMsgToChannel(channelId string, msg string, replyToId string) {
	channel, resp := client.GetChannel(channelId, "")
	if resp.Error != nil {
		println("There was a problem getting channel")
		PrintError(resp.Error)
	}

	post := &model.Post{}
	post.ChannelId = channelId
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		println("We failed to send a message to the channel " + channel.Name)
		PrintError(resp.Error)
	}
}

func SendMsgToDebuggingChannel(msg string, replyToId string) {
	post := &model.Post{}
	post.ChannelId = debuggingChannel.Id
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
}

func SetToken() {
	client.SetToken(BOT_TOKEN_SECRET)
}

func PrintError(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
}

func GetBotUser() {
	if user, resp := client.GetUser(BOT_ID, ""); resp.Error != nil {
		println("There was a problem getting Bot User")
		PrintError(resp.Error)
	} else {
		botUser = user
	}
}

func SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			if webSocketClient != nil {
				webSocketClient.Close()
			}

			SendMsgToDebuggingChannel("bot "+botUser.Username+" has **stopped** running", "")
			os.Exit(0)
		}
	}()
}
