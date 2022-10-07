package event

import (
	"bytes"
	"encoding/gob"
	"github.com/nats-io/go-nats"
	"textBoard/schema"
)

type NatsEventStore struct {
	nc                      *nats.Conn
	postCreatedSubscription *nats.Subscription
	postCreatedChan         chan PostCreatedMessage
}

func NewNats(url string) (*NatsEventStore, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NatsEventStore{nc: nc}, nil
}

func (e *NatsEventStore) Close() {
	if e.nc != nil {
		e.nc.Close()
	}
	if e.postCreatedSubscription != nil {
		e.postCreatedSubscription.Unsubscribe()
	}
	close(e.postCreatedChan)
}

func (e *NatsEventStore) PublishPostCreated(post schema.Post) error {
	m := PostCreatedMessage{post.ID, post.Body, post.CreatedAt}
	data, err := e.writeMessage(&m)
	if err != nil {
		return err
	}
	return e.nc.Publish(m.Key(), data)
}

func (mq *NatsEventStore) writeMessage(m Message) ([]byte, error) {
	b := bytes.Buffer{}
	err := gob.NewEncoder(&b).Encode(m)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (e *NatsEventStore) OnPostCreated(f func(PostCreatedMessage)) (err error) {
	m := PostCreatedMessage{}
	e.postCreatedSubscription, err = e.nc.Subscribe(m.Key(), func(msg *nats.Msg) {
		e.readMessage(msg.Data, &m)
		f(m)
	})
	return
}

func (mq *NatsEventStore) readMessage(data []byte, m interface{}) error {
	b := bytes.Buffer{}
	b.Write(data)
	return gob.NewDecoder(&b).Decode(m)
}

func (e *NatsEventStore) SubscribePostCreated() (<-chan PostCreatedMessage, error) {
	m := PostCreatedMessage{}
	e.postCreatedChan = make(chan PostCreatedMessage, 64)
	ch := make(chan *nats.Msg, 64)
	var err error
	e.postCreatedSubscription, err = e.nc.ChanSubscribe(m.Key(), ch)
	if err != nil {
		return nil, err
	}
	// Decode message
	go func() {
		for {
			select {
			case msg := <-ch:
				e.readMessage(msg.Data, &m)
				e.postCreatedChan <- m
			}
		}
	}()
	return (<-chan PostCreatedMessage)(e.postCreatedChan), nil
}
