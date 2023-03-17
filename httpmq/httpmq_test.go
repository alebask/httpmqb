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
		t.Errorf("after timeout expired, expected status %v, received %v", resp.StatusCode, http.StatusNotFound)
	}
}
