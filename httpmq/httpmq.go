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
	"sync"
	"time"
)

type pushMessage struct {
	topic string
	value string
}
type popMessage struct {
	topic   string
	valueCh chan string
	done    chan struct{}
}

// listener pairs a consumer's response channel with a cancellation signal.
// done is closed by the consumer when it times out, so the event loop can
// skip it rather than delivering to a goroutine that has already returned.
type listener struct {
	valueCh chan string
	done    chan struct{}
}

type topic struct {
	items     *queue.Queue[string]
	listeners *queue.Queue[listener]
}

func (t *topic) pop(lsr listener) {
	value, ok := t.items.Pop()
	if !ok {
		t.listeners.Push(lsr)
	} else {
		lsr.valueCh <- value // buffered(1): always succeeds, no race with consumer start
	}
}

func (t *topic) push(value string) {
	for {
		lsr, ok := t.listeners.Pop()
		if !ok {
			t.items.Push(value)
			return
		}
		// Skip listeners that already timed out.
		select {
		case <-lsr.done:
			continue
		default:
		}
		// Deliver; handle the case where the listener cancels simultaneously.
		select {
		case lsr.valueCh <- value:
			return
		case <-lsr.done:
			continue
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
			listeners: queue.New[listener](),
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
			t.pop(listener{valueCh: msg.valueCh, done: msg.done})
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
func (mq *httpmq) pop(ctx context.Context, topic string, timeout int) (string, bool) {
	valueCh := make(chan string, 1)
	done := make(chan struct{})
	mq.popCh <- popMessage{topic: topic, valueCh: valueCh, done: done}

	// A nil timer channel never fires; used for the infinite-wait (timeout==0) case.
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(time.Duration(timeout) * time.Second)
	}

	select {
	case value := <-valueCh:
		return value, true
	case <-timer:
		close(done)
		// Drain in case a value was delivered concurrently with the timeout.
		select {
		case value := <-valueCh:
			return value, true
		default:
			return "", false
		}
	case <-ctx.Done():
		close(done)
		// Drain in case a value was delivered concurrently with cancellation.
		select {
		case value := <-valueCh:
			return value, true
		default:
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
	if err != nil && r.URL.Query().Has("timeout") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	value, ok := mq.pop(r.Context(), topicName, timeout)

	if !ok {
		w.WriteHeader(http.StatusNotFound)
	} else {
		if _, err := w.Write([]byte(value)); err != nil {
			logger.Warning("failed to write response", logger.Fields{"error": err})
		}
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

	srv := http.Server{Addr: ":" + strconv.Itoa(port), Handler: mq}

	done := make(chan struct{})
	var once sync.Once
	shutdown := func() {
		once.Do(func() { close(done) })
	}
	defer shutdown()

	go mq.start(done)

	go func() {
		var s string
		for s != "q" {
			fmt.Scanln(&s)
		}
		shutdown()
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Error("http server shutdown error", logger.Fields{"error": err, "port": port})
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		logger.Info("httpmbq shutting down after ctrl-c", logger.Fields{"port": port})
		shutdown()
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Error("http server shutdown error", logger.Fields{"error": err, "port": port})
		}
	}()

	return srv.ListenAndServe()
}
