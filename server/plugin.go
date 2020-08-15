package main

import (
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	botUserID string
}

// OnActivate activate plugin
func (p *Plugin) OnActivate() error {
	bot := &model.Bot{
		Username:    "random-user",
		DisplayName: "RandomUser",
	}
	botUserID, ensureBotErr := p.Helpers.EnsureBot(bot)

	if ensureBotErr != nil {
		return ensureBotErr
	}

	p.botUserID = botUserID

	return p.API.RegisterCommand(&model.Command{
		Trigger:          "random-user",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: user, users, user-here, users-here",
	})
}

func shuffleUsers(a []*model.User) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
}

func isOnline(usersStatus []*model.Status, userID string) bool {
	for i := range usersStatus {
		if usersStatus[i].UserId == userID {
			return usersStatus[i].Status != "offline"
		}
	}

	return false
}

func (p *Plugin) filterOfflineUsers(users []*model.User) []*model.User {
	var userIds []string
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}

	usersStatus, _ := p.API.GetUserStatusesByIds(userIds)

	var onlineUsers []*model.User

	for i := range users {
		if isOnline(usersStatus, users[i].Id) {
			onlineUsers = append(onlineUsers, users[i])
		}
	}

	return onlineUsers
}

func (p *Plugin) filterBots(users []*model.User) []*model.User {
	var noBots []*model.User

	for _, user := range users {
		if !user.IsBot {
			noBots = append(noBots, user)
		}
	}

	return noBots
}

// ExecuteCommand run command
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	msg := ""
	action := ""

	if len(split) > 1 {
		action = split[1]
	}

	users, _ := p.API.GetUsersInChannel(args.ChannelId, "username", 0, 1000)

	users = p.filterBots(users)

	if action == "user-here" || action == "users-here" {
		users = p.filterOfflineUsers(users)
	}

	if len(users) > 0 {
		if action == "users" || action == "users-here" {
			shuffleUsers(users)
			var usernames []string

			for _, user := range users {
				usernames = append(usernames, "@"+user.Username)
			}

			msg = strings.Join(usernames, ", ")
		} else {
			usersLen := len(users)
			userIndex := rand.Intn(usersLen)
			msg = "@" + users[userIndex].Username
		}

		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: args.ChannelId,
			RootId:    args.RootId,
			Message:   msg,
		}

		_, createPostError := p.API.CreatePost(post)

		if createPostError != nil {
			return nil, model.NewAppError("ExecuteCommand", "error random-user", nil, createPostError.Error(), http.StatusInternalServerError)
		}
	}

	return &model.CommandResponse{}, nil
}
