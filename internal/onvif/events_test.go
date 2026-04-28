package onvif

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aragarwal/onvif-server/internal/config"
)

func TestHandleSubscribe(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/event_service", nil)
	r.Host = "10.0.0.5:8081"
	w := httptest.NewRecorder()

	s.handleSubscribe(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"SubscribeResponse",
		"http://10.0.0.5:8081/onvif/subscription/events",
		"<wsa5:Address>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q:\n%s", want, body)
		}
	}
}

func TestHandleGetEventProperties(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam"}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/event_service", nil)
	w := httptest.NewRecorder()

	s.handleGetEventProperties(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"GetEventPropertiesResponse",
		"http://www.onvif.org/onvif/ver10/topics/topicns.xml",
		"<wsnt:FixedTopicSet>true</wsnt:FixedTopicSet>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q:\n%s", want, body)
		}
	}
}

func TestHandleCreatePullPointSubscription(t *testing.T) {
	s := newTestServer(config.CameraConfig{Name: "cam", HTTPPort: 8081}, "admin", "admin")
	r := httptest.NewRequest("POST", "/onvif/event_service", nil)
	r.Host = "10.0.0.5:8081"
	w := httptest.NewRecorder()

	s.handleCreatePullPointSubscription(w, r)
	body := w.Body.String()

	for _, want := range []string{
		"CreatePullPointSubscriptionResponse",
		"http://10.0.0.5:8081/onvif/subscription/pullpoint",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q:\n%s", want, body)
		}
	}
}
