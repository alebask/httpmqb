package httpmq

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPushPopProperSequence(t *testing.T) {

	N := 2
	expected := make([]string, N)
	queueName := "test"

	mq := &httpmq{
		pushCh: make(chan pushMessage, N),
		popCh:  make(chan popMessage, N),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)

	go mq.start(done)

	for i := 0; i < N; i++ {
		expected[i] = fmt.Sprintf("value_%v", i)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/%v?v=%v", queueName, expected[i]), nil)
		w := httptest.NewRecorder()
		mq.pushHandler(w, req)
	}

	for i := 0; i < N; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%v", queueName), nil)
		w := httptest.NewRecorder()
		mq.popHandler(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		actual := string(body)

		if actual != expected[i] {
			t.Errorf("pushed %v, but received %v", expected[i], actual)
		}
	}
}

func TestPopTimeoutNotExpired(t *testing.T) {

	queueName := "test"
	timeout := 100

	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	actualValueCh := make(chan string, 1)
	actualStatusCh := make(chan int, 1)

	go func(timeout int) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%v?timeout=%v", queueName, timeout), nil)
		w := httptest.NewRecorder()
		mq.popHandler(w, req)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		actualValueCh <- string(body)
		actualStatusCh <- resp.StatusCode

	}(timeout)

	expected := "expected_value"
	req := httptest.NewRequest("PUT", fmt.Sprintf("/%v?v=%v", queueName, expected), nil)
	w := httptest.NewRecorder()
	mq.pushHandler(w, req)

	actualValue := <-actualValueCh
	actualStatusCode := <-actualStatusCh

	if actualValue != expected {
		t.Errorf("pushed %v, but received %v", expected, actualValue)
	}
	if actualStatusCode != http.StatusOK {
		t.Errorf("expected status %v, received status %v", http.StatusOK, actualStatusCode)
	}
}

func TestPopTimeoutExpired(t *testing.T) {
	queueName := "test"
	timeout := 1

	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	req := httptest.NewRequest("GET", fmt.Sprintf("/%v?timeout=%v", queueName, timeout), nil)
	w := httptest.NewRecorder()
	mq.popHandler(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("after timeout expired, expected status %v, received %v", http.StatusNotFound, resp.StatusCode)
	}
}

func TestPushHandlerMissingTopic(t *testing.T) {
	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	req := httptest.NewRequest("PUT", "/?v=hello", nil)
	w := httptest.NewRecorder()
	mq.pushHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %v, got %v", http.StatusBadRequest, w.Code)
	}
}

func TestPushHandlerMissingValue(t *testing.T) {
	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	req := httptest.NewRequest("PUT", "/mytopic", nil)
	w := httptest.NewRecorder()
	mq.pushHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %v, got %v", http.StatusBadRequest, w.Code)
	}
}

func TestPopHandlerMissingTopic(t *testing.T) {
	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mq.popHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %v, got %v", http.StatusBadRequest, w.Code)
	}
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	for _, method := range []string{"POST", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/mytopic", nil)
		w := httptest.NewRecorder()
		mq.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %v: expected status %v, got %v", method, http.StatusMethodNotAllowed, w.Code)
		}
	}
}

func TestServeHTTPRoutesPutAndGet(t *testing.T) {
	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	putReq := httptest.NewRequest("PUT", "/mytopic?v=hello", nil)
	putW := httptest.NewRecorder()
	mq.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Errorf("PUT: expected status %v, got %v", http.StatusOK, putW.Code)
	}

	getReq := httptest.NewRequest("GET", "/mytopic?timeout=1", nil)
	getW := httptest.NewRecorder()
	mq.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Errorf("GET: expected status %v, got %v", http.StatusOK, getW.Code)
	}
	if body := getW.Body.String(); body != "hello" {
		t.Errorf("GET: expected body %q, got %q", "hello", body)
	}
}

func TestPopInfiniteWait(t *testing.T) {
	queueName := "test"

	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	resultCh := make(chan string, 1)
	go func() {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%v?timeout=0", queueName), nil)
		w := httptest.NewRecorder()
		mq.popHandler(w, req)
		resultCh <- w.Body.String()
	}()

	expected := "infinite_wait_value"
	req := httptest.NewRequest("PUT", fmt.Sprintf("/%v?v=%v", queueName, expected), nil)
	w := httptest.NewRecorder()
	mq.pushHandler(w, req)

	actual := <-resultCh
	if actual != expected {
		t.Errorf("pushed %v, but received %v", expected, actual)
	}
}

func TestHundredTopicsIsolation(t *testing.T) {
	N := 100

	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	// Push one message to each of N distinct topics.
	for i := 0; i < N; i++ {
		topicName := fmt.Sprintf("topic_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mq.pushHandler(
			httptest.NewRecorder(),
			httptest.NewRequest("PUT", fmt.Sprintf("/%s?v=%s", topicName, value), nil),
		)
	}

	// Pop from each topic and verify the correct message is returned,
	// confirming topics are fully isolated from one another.
	for i := 0; i < N; i++ {
		topicName := fmt.Sprintf("topic_%d", i)
		expected := fmt.Sprintf("value_%d", i)

		w := httptest.NewRecorder()
		mq.popHandler(w, httptest.NewRequest("GET", fmt.Sprintf("/%s?timeout=1", topicName), nil))

		if w.Code != http.StatusOK {
			t.Errorf("topic %s: expected status %v, got %v", topicName, http.StatusOK, w.Code)
		}
		if body := w.Body.String(); body != expected {
			t.Errorf("topic %s: expected %q, got %q", topicName, expected, body)
		}
	}
}

func TestPushSkipsDeadListener(t *testing.T) {
	queueName := "test"

	mq := &httpmq{
		pushCh: make(chan pushMessage, 1),
		popCh:  make(chan popMessage, 1),
		topics: make(map[string]*topic),
	}

	done := make(chan struct{})
	defer close(done)
	go mq.start(done)

	// Register a listener that times out — its done channel is closed on return,
	// so the event loop will skip it on the next push.
	mq.popHandler(httptest.NewRecorder(), httptest.NewRequest("GET", fmt.Sprintf("/%v?timeout=1", queueName), nil))

	// Push — event loop must skip the cancelled listener and store in queue.
	mq.pushHandler(httptest.NewRecorder(), httptest.NewRequest("PUT", fmt.Sprintf("/%v?v=after_timeout", queueName), nil))

	// Fresh GET should retrieve the stored message.
	getW := httptest.NewRecorder()
	mq.popHandler(getW, httptest.NewRequest("GET", fmt.Sprintf("/%v?timeout=1", queueName), nil))

	if getW.Code != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, getW.Code)
	}
	if body := getW.Body.String(); body != "after_timeout" {
		t.Errorf("expected %q, got %q", "after_timeout", body)
	}
}
