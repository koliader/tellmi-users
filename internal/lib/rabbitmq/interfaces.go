package rabbitmq

type MessageSender interface {
	SendMessage(queueName string, message []byte) error
}
