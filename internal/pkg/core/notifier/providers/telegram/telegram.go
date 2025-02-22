﻿package telegram

import (
	"context"

	"github.com/nikoksr/notify/service/telegram"

	"github.com/usual2970/certimate/internal/pkg/core/notifier"
)

type NotifierConfig struct {
	// Telegram API Token。
	ApiToken string `json:"apiToken"`
	// Telegram Chat ID。
	ChatId int64 `json:"chatId"`
}

type NotifierProvider struct {
	config *NotifierConfig
}

var _ notifier.Notifier = (*NotifierProvider)(nil)

func NewNotifier(config *NotifierConfig) (*NotifierProvider, error) {
	if config == nil {
		panic("config is nil")
	}

	return &NotifierProvider{
		config: config,
	}, nil
}

func (n *NotifierProvider) Notify(ctx context.Context, subject string, message string) (res *notifier.NotifyResult, err error) {
	srv, err := telegram.New(n.config.ApiToken)
	if err != nil {
		return nil, err
	}

	srv.AddReceivers(n.config.ChatId)

	err = srv.Send(ctx, subject, message)
	if err != nil {
		return nil, err
	}

	return &notifier.NotifyResult{}, nil
}
