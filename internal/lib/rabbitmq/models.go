package rabbitmq

type UserUpdated struct {
	Username    string `json:"username"`
	NewUsername string `json:"newUsername"`
}

type UserCreated struct {
	Username string `json:"username"`
}
