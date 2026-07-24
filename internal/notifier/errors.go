package notifier

import "errors"

var ErrUnknownOperationType = errors.New("unknown operation type")

// имя consumer-группы: по нему Kafka хранит оффсеты этого сервиса.
const consumerGroup = "notifier"
