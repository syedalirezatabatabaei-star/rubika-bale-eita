package balemeow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mimalef70/goomni/src/internal/balemeow/wire"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestOfficialAuthLifetimeAndCooldownAreDifferentFields(t *testing.T) {
	// Encode independent tags, rather than mirroring generated struct field names.
	wrapper := func(v uint64) []byte {
		return protowire.AppendVarint(protowire.AppendTag(nil, 1, protowire.VarintType), v)
	}
	raw := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "synthetic-transaction")
	raw = protowire.AppendBytes(protowire.AppendTag(raw, 8, protowire.BytesType), wrapper(60))
	raw = protowire.AppendBytes(protowire.AppendTag(raw, 9, protowire.BytesType), wrapper(120))
	var response wire.StartPhoneAuthResponse
	if e := decode(raw, &response); e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1720000000, 0)
	m, e := authMetadata(&response, now)
	if e != nil {
		t.Fatal(e)
	}
	if m.ResendAfterSeconds == nil || *m.ResendAfterSeconds != 60 || !m.ExpiresAt.Equal(now.Add(120*time.Second)) {
		t.Fatalf("wrong auth timing: %+v", m)
	}
	if b, _ := json.Marshal(m); strings.Contains(string(b), "synthetic-transaction") {
		t.Fatal("provider transaction exposed")
	}
	response.CodeTimeout = &wire.Int32Value{Value: -1}
	if _, e := authMetadata(&response, now); e == nil {
		t.Fatal("negative TTL accepted")
	}
	response.CodeTimeout = &wire.Int32Value{Value: 86400}
	m, e = authMetadata(&response, now)
	if e != nil || !m.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatal("local bound lost")
	}
	response.NextSendCodeWaitTime = &wire.Int64Value{Value: 1 << 60}
	if _, e := authMetadata(&response, now); e == nil {
		t.Fatal("overflow cooldown accepted")
	}
}
func TestOfficialPasswordPhraseThroughUnaryAuthFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc-web+proto")
		if strings.HasSuffix(r.URL.Path, "StartPhoneAuth") {
			_, _ = w.Write(grpcFrame(0, marshal(t, &wire.StartPhoneAuthResponse{TransactionHash: "fake", CodeTimeout: &wire.Int32Value{Value: 120}})))
			_, _ = w.Write(grpcFrame(128, []byte("grpc-status: 0\r\n")))
			return
		}
		phrase := "password needed for login"
		if strings.HasSuffix(r.URL.Path, "ValidatePassword") {
			phrase = "wrong password"
		}
		_, _ = w.Write(grpcFrame(128, []byte("grpc-status: 16\r\ngrpc-message: "+phrase+"\r\n")))
	}))
	defer server.Close()
	c := New(Options{GRPCEndpoint: server.URL, AppID: 99, APIKey: "synthetic"})
	ch, e := c.StartAuth(context.Background(), "09123456789")
	if e != nil {
		t.Fatal(e)
	}
	session, e := c.SubmitCode(context.Background(), ch.ID, "12345")
	if e != nil || session != nil || c.Status().Auth != "awaiting_password" {
		t.Fatalf("2FA transition: session=%v err=%v status=%v", session != nil, e, c.Status())
	}
	if _, e = c.SubmitPassword(context.Background(), ch.ID, "  preserve password spaces  "); codeOf(e) != "INVALID_PASSWORD" {
		t.Fatalf("wrong-password mapping: %v", e)
	}
	if codeOf(authProviderError(16, "password needed for login user-secret")) == "PASSWORD_REQUIRED" {
		t.Fatal("untrusted arbitrary suffix accepted")
	}
}

func TestStartPhoneAuthUsesReferenceWireShape(t *testing.T) {
	var gotDeviceHash []byte
	var gotUnknown []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil || len(raw) < 5 || raw[0] != 0 {
			t.Fatalf("invalid gRPC-Web frame: %v", err)
		}
		var req wire.StartPhoneAuthRequest
		if err := proto.Unmarshal(raw[5:], &req); err != nil {
			t.Fatal(err)
		}
		gotDeviceHash = append([]byte(nil), req.DeviceHash...)
		gotUnknown = append([]byte(nil), req.ProtoReflect().GetUnknown()...)
		_, _ = w.Write(grpcFrame(0, marshal(t, &wire.StartPhoneAuthResponse{TransactionHash: "synthetic-tx"})))
		_, _ = w.Write(grpcFrame(128, []byte("grpc-status: 0\\r\\n")))
	}))
	defer server.Close()
	c := New(Options{GRPCEndpoint: server.URL, AppID: 4, APIKey: "synthetic"})
	if _, err := c.StartAuth(context.Background(), "09123456789"); err != nil {
		t.Fatal(err)
	}
	if len(gotDeviceHash) != 16 {
		t.Fatalf("device_hash length = %d, want 16 raw bytes", len(gotDeviceHash))
	}
	if len(gotUnknown) != 0 {
		t.Fatalf("unexpected unknown StartPhoneAuth request fields: %x", gotUnknown)
	}
}

func TestSafeGRPCMessageClassDoesNotExposeFreeFormText(t *testing.T) {
	cases := map[string]string{
		"PHONE_NUMBER_INVALID": "PHONE_NUMBER_INVALID",
		"phone number invalid": "PHONE_NUMBER_INVALID",
		"invalid phone number +989121234567": "UNCLASSIFIED",
		"bad request token=secret": "UNCLASSIFIED",
		"": "UNCLASSIFIED",
	}
	for input, want := range cases {
		if got := safeGRPCMessageClass(input); got != want {
			t.Errorf("safeGRPCMessageClass(%q) = %q, want %q", input, got, want)
		}
	}
}
