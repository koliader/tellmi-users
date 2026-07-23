package rabbitmq

import (
	"fmt"

	"github.com/koliader/tellmi-users/internal/lib/config"
	"github.com/streadway/amqp"
)

var UserUpdatedQueue = "updateUser"
var UserCreatedQueue = "userCreated"

type Client struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewRabbitmqClient(config config.Config) (*Client, error) {
	conn, err := amqp.Dial(config.RbmUrl)
	if err != nil {
		return nil, fmt.Errorf("error connecting to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("error creating RabbitMQ channel: %w", err)
	}

	return &Client{
		conn:    conn,
		Channel: channel,
	}, nil
}

func (c *Client) CreateQueue(queueName string) error {
	if c == nil || c.Channel == nil {
		return fmt.Errorf("rabbitmq client or channel is nil")
	}
	_, err := c.Channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error declaring queue on RabbitMQ: %w", err)
	}
	return nil
}

func (c *Client) SendMessage(queueName string, message []byte) error {
	if c == nil || c.Channel == nil {
		return fmt.Errorf("rabbitmq client or channel is nil")
	}

	_, err := c.Channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error declaring queue on RabbitMQ: %w", err)
	}

	err = c.Channel.Publish(
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        message,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing message to RabbitMQ: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.Channel != nil {
		if err := c.Channel.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ channel: %w", err)
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ connection: %w", err)
		}
	}
	return nil
}
