package actor

import (
	"time"

	dapr "github.com/dapr/go-sdk/client"
)

func NewDapr() (dapr.Client, error) {
	var client dapr.Client
	var err error
	for attempts := 120; attempts > 0; attempts-- {
		client, err = dapr.NewClient()
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	return client, err
}
