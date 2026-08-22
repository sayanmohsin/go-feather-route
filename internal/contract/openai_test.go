package contract

import "testing"

func TestDecodeChatRequestPreservesRequiredFields(t *testing.T) {
	request, err := DecodeChatRequest([]byte(`{"model":"test","stream":true,"messages":[],"response_format":{"type":"json_object"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != "test" || !request.Stream || request.ResponseFormat == nil || request.ResponseFormat.Type != "json_object" {
		t.Fatalf("request=%+v", request)
	}
}

func TestEmbeddingInputCount(t *testing.T) {
	request, err := DecodeEmbeddingRequest([]byte(`{"model":"embed","input":["one","two"]}`))
	if err != nil {
		t.Fatal(err)
	}
	count, err := request.InputCount()
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestEmbeddingRequestRejectsEmptyBatch(t *testing.T) {
	request, err := DecodeEmbeddingRequest([]byte(`{"model":"embed","input":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.InputCount(); err == nil {
		t.Fatal("expected empty batch error")
	}
}
