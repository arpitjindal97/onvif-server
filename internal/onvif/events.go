package onvif

import (
	"encoding/xml"
	"net/http"
	"time"

	"github.com/aragarwal/onvif-server/internal/logger"
)

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)
	subscriptionAddr := baseURL + "/onvif/subscription/events"

	now := time.Now().UTC()
	terminationTime := now.Add(1 * time.Hour)

	response := subscribeResponse{
		SubscriptionReference: subscriptionReference{Address: subscriptionAddr},
		CurrentTime:           now.Format(time.RFC3339),
		TerminationTime:       terminationTime.Format(time.RFC3339),
	}

	envelope := struct {
		XMLName xml.Name `xml:"SOAP-ENV:Envelope"`
		SOAPENV string   `xml:"xmlns:SOAP-ENV,attr"`
		WSNT    string   `xml:"xmlns:wsnt,attr"`
		WSA5    string   `xml:"xmlns:wsa5,attr"`
		Body    struct {
			XMLName xml.Name    `xml:"SOAP-ENV:Body"`
			Content interface{} `xml:",any"`
		} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		WSNT:    "http://docs.oasis-open.org/wsn/b-2",
		WSA5:    "http://www.w3.org/2005/08/addressing",
	}
	envelope.Body.Content = response

	output, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		logger.Info("[%s] Failed to marshal response: %v", s.config.Name, err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseStr := xml.Header + string(output)
	logger.Debug("[%s] Sending Response:\n%s", s.config.Name, responseStr)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseStr))
}

func (s *Server) handleGetEventProperties(w http.ResponseWriter, r *http.Request) {
	response := getEventPropertiesResponse{
		TopicNamespaceLocation: []string{
			"http://www.onvif.org/onvif/ver10/topics/topicns.xml",
		},
		FixedTopicSet: true,
	}
	s.sendSOAPResponse(w, response)
}

func (s *Server) handleCreatePullPointSubscription(w http.ResponseWriter, r *http.Request) {
	baseURL := s.getBaseURL(r)
	subscriptionAddr := baseURL + "/onvif/subscription/pullpoint"

	now := time.Now().UTC()
	terminationTime := now.Add(1 * time.Hour)

	response := createPullPointSubscriptionResponse{
		SubscriptionReference: subscriptionReference{Address: subscriptionAddr},
		CurrentTime:           now.Format(time.RFC3339),
		TerminationTime:       terminationTime.Format(time.RFC3339),
	}

	s.sendSOAPResponse(w, response)
}
