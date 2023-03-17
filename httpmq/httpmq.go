package httpmq

import (
	"context"
	"fmt"
	"httpmqb/logger"
	"httpmqb/queue"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

type pushMessage struct {
	topic string
	value string
}
type popMessage struct {
	topic   string
	valueCh chan string
}
type topic struct {
	items     *queue.Queue[string]
	listeners *queue.Queue[chan string]
}

func (t *topic) pop(out chan string) {

	value, ok := t.items.Pop()
	if !ok {
		t.listeners.Push(out)
	} else {
		select {
		case out <- value:
		default:
			t.listeners.Push(out)
		}
	}
}
func (t *topic) push(value string) {
	for {
		lsr, ok := t.listeners.Pop()
		if !ok {
			t.items.Push(value)
			return
		} else {
			select {
			case lsr <- value:
				return
			default:
				continue
			}
		}
	}
}

type httpmq struct {
	topics map[string]*topic
	pushCh chan pushMessage
	popCh  chan popMessage
}

func New() *httpmq {
	return &httpmq{
		topics: make(map[string]*topic),
		pushCh: make(chan pushMessage),
		popCh:  make(chan popMessage),
	}
}

func (mq *httpmq) getOrCreateTopic(name string) *topic {
	t, ok := mq.topics[name]
	if !ok {
		t = &topic{
			items:     queue.New[string](),
			listeners: queue.New[chan string](),
		}
		mq.topics[name] = t
	}
	return t
}

func (mq *httpmq) start(done <-chan struct{}) {
	for {
		select {
		case msg := <-mq.pushCh:
			t := mq.getOrCreateTopic(msg.topic)
			t.push(msg.value)
		case msg := <-mq.popCh:
			t := mq.getOrCreateTopic(msg.topic)
			t.pop(msg.valueCh)
		case <-done:
			return
		}
	}
}

func (mq *httpmq) push(topic, value string) {
	mq.pushCh <- pushMessage{
		topic: topic,
		value: value,
	}
}
func (mq *httpmq) pop(topic string, timeout int) (string, bool) {

	valueCh := make(chan string)
	mq.popCh <- popMessage{topic: topic, valueCh: valueCh}

	if timeout == 0 {
		value := <-valueCh
		return value, true
	} else {
		select {
		case value := <-valueCh:
			return value, true
		case <-time.After(time.Duration(timeout) * time.Second):
			return "", false
		}
	}
}
func (mq *httpmq) pushHandler(w http.ResponseWriter, r *http.Request) {
	topicName := strings.TrimPrefix(r.URL.Path, "/")
	if topicName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	value := r.URL.Query().Get("v")
	if value == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mq.push(topicName, value)

	w.WriteHeader(http.StatusOK)
}
func (mq *httpmq) popHandler(w http.ResponseWriter, r *http.Request) {
	topicName := strings.TrimPrefix(r.URL.Path, "/")
	if topicName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	timeout, err := strconv.Atoi(r.URL.Query().Get("timeout"))
	if err != nil {
		timeout = 0
	}

	value, ok := mq.pop(topicName, timeout)

	if !ok {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.Write([]byte(value))
	}
}
func (mq *httpmq) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodPut:
		mq.pushHandler(w, r)
	case http.MethodGet:
		mq.popHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (mq *httpmq) ListenAndServe(port int) error {

	srv := http.Server{Addr: ":" + strconv.Itoa(port)}
	http.Handle("/", mq)

	done := make(chan struct{})
	defer close(done)

	go mq.start(done)

	go func() {
		var s string
		for s != "q" {
			fmt.Scanln(&s)
		}
		done <- struct{}{}
		err := srv.Shutdown(context.Background())
		if err != nil {
			logger.Error("http server shutdown error", logger.Fields{"error": err, "port": port})
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		logger.Info("httpmbq shutting down after ctrl-c", logger.Fields{"port": port})
		done <- struct{}{}
		srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}
