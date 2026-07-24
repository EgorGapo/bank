package usecases

import "context"

type NotifierStorage interface {
	InsertNotification(ctx context.Context, eventId, accountId, text string) error
}

type notifier struct {
	storage NotifierStorage
}

func NewNotifier(storage NotifierStorage) *notifier {
	return &notifier{
		storage: storage,
	}
}

func (n *notifier) InsertNotification(ctx context.Context, eventId, accountId, text string) error {
	return n.storage.InsertNotification(ctx, eventId, accountId, text)
}
