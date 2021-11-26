package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hako/durafmt"
	"github.com/prometheus/alertmanager/notify/webhook"

	"github.com/mattermost/mattermost-server/v6/model"
)

func (p *Plugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	p.API.LogInfo("Received alertmanager notification")

	var message webhook.Message
	err := json.NewDecoder(r.Body).Decode(&message)
	if err != nil {
		p.API.LogError("failed to decode webhook message", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var fields []*model.SlackAttachmentField
	for _, alert := range message.Alerts {
		statusMsg := strings.ToUpper(alert.Status)
		if alert.Status == "firing" {
			statusMsg = fmt.Sprintf(":fire: %s :fire:", strings.ToUpper(alert.Status))
		}

		/* first field: Annotations, Start/End, Source */
		var msg string
		for k, v := range alert.Annotations {
			msg = fmt.Sprintf("%s**%s:** %s\n", msg, strings.Title(k), v)
		}
		msg = fmt.Sprintf("%s \n", msg)
		msg = fmt.Sprintf("%s**Started at:** %s (%s ago)\n", msg,
			(alert.StartsAt).Format(time.RFC1123),
			durafmt.Parse(time.Since(alert.StartsAt)).LimitFirstN(2).String(),
		)
		if alert.Status == "resolved" {
			msg = fmt.Sprintf("%s**Ended at:** %s (%s ago)\n", msg,
				(alert.EndsAt).Format(time.RFC1123),
				durafmt.Parse(time.Since(alert.EndsAt)).LimitFirstN(2).String(),
			)
		}
		msg = fmt.Sprintf("%s \n", msg)
		msg = fmt.Sprintf("%sGenerated by a [Prometheus Alert](%s) and sent to the [Alertmanager](%s) '%s' receiver.", msg, alert.GeneratorURL, message.ExternalURL, message.Receiver)
		fields = addFields(fields, statusMsg, msg, true)

		/* second field: Labels only */
		msg = ""
		for k, v := range alert.Labels {
			msg = fmt.Sprintf("%s**%s:** %s\n", msg, strings.Title(k), v)
		}
		fields = append(fields, &model.SlackAttachmentField{
			Value: msg,
			Short: model.SlackCompatibleBool(true),
		})
	}

	attachment := &model.SlackAttachment{
		Fields: fields,
		Color:  setColor(message.Status),
	}

	post := &model.Post{
		ChannelId: p.ChannelID,
		UserId:    p.BotUserID,
	}

	model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	if _, appErr := p.API.CreatePost(post); appErr != nil {
		return
	}
}

func addFields(fields []*model.SlackAttachmentField, title, msg string, short bool) []*model.SlackAttachmentField {
	return append(fields, &model.SlackAttachmentField{
		Title: title,
		Value: msg,
		Short: model.SlackCompatibleBool(short),
	})
}

func setColor(impact string) string {
	mapImpactColor := map[string]string{
		"firing":   "#FF0000",
		"resolved": "#008000",
	}

	if val, ok := mapImpactColor[impact]; ok {
		return val
	}

	return "#F0F8FF"
}
