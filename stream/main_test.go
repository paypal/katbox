package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowseHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/browse", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", ".")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(browseHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestReadHandlerWIthVakidOffset(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/read", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", "/Users/revchandra/Desktop/go/dce.err")
	q.Add("offset", "0")
	q.Add("length", "1000")
	q.Add("jsonp", "jQuery17107124409226948478_1614562818140&_=1614562818159")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(readHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestReadHandlerWithOffsetOverflow(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/read", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", ".")
	q.Add("offset", "4174")
	q.Add("length", "1000")
	q.Add("jsonp", "jQuery17107124409226948478_1614562818140&_=1614562818159")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(readHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestReadHandlerWithInvalidOffset(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/read", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", ".")
	q.Add("offset", "-1")
	q.Add("length", "-1")
	q.Add("jsonp", "jQuery17107124409226948478_1614562818140&_=1614562818159")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(readHandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}
func TestDownloadHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/download", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", ".")

	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(downloadhandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestDownloadHandlerWithInvalidPath(t *testing.T) {
	req, err := http.NewRequest("GET", "/files/download", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("path", ".")

	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(downloadhandler)
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
