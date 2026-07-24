package notifier

import "context"

func (n *Notifier) consumeLoop(ctx context.Context) error {
	for {
		records, err := n.consumer.Poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				n.logger.Info("notifier stopped")
				return nil
			}
			n.logger.Error("poll failed", "error", err)
			continue
		}

		for _, r := range records {
			if err := n.process(ctx, r.Key, r.Value); err != nil {
				// TODO(этап 4): вместо пропуска — отправлять в DLQ (ledger.operations.dlq)
				n.logger.Warn("skip record", "error", err, "offset", r.Offset)
			}
		}

		if err := n.consumer.Commit(ctx, records...); err != nil {
			n.logger.Error("commit failed", "error", err)
		}
	}
}
