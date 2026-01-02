package protocol

import (
	"encoding/json"
	"testing"
)

func TestRequest_Serialize(t *testing.T) {
	req := Request{Method: MethodPing}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Method != MethodPing {
		t.Errorf("got method %q, want %q", decoded.Method, MethodPing)
	}
}

func TestResponse_Success(t *testing.T) {
	resp := SuccessResponse("pong")
	if resp.Error != "" {
		t.Errorf("expected no error, got %q", resp.Error)
	}

	var result string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result != "pong" {
		t.Errorf("got result %q, want %q", result, "pong")
	}
}

func TestResponse_Error(t *testing.T) {
	resp := ErrorResponse("something failed")
	if resp.Error != "something failed" {
		t.Errorf("got error %q, want %q", resp.Error, "something failed")
	}
	if resp.Result != nil {
		t.Errorf("expected nil result, got %v", resp.Result)
	}
}

func TestSessionsParams(t *testing.T) {
	params := SessionsParams{IncludeRestorePoints: true}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SessionsParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !decoded.IncludeRestorePoints {
		t.Error("IncludeRestorePoints should be true")
	}
}
