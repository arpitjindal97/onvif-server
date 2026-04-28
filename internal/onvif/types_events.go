package onvif

import "encoding/xml"

// Event service types.
type subscribeResponse struct {
	XMLName               xml.Name              `xml:"wsnt:SubscribeResponse"`
	SubscriptionReference subscriptionReference `xml:"wsnt:SubscriptionReference"`
	CurrentTime           string                `xml:"wsnt:CurrentTime,omitempty"`
	TerminationTime       string                `xml:"wsnt:TerminationTime,omitempty"`
}

type subscriptionReference struct {
	Address string `xml:"wsa5:Address"`
}

type getEventPropertiesResponse struct {
	XMLName                xml.Name `xml:"tev:GetEventPropertiesResponse"`
	TopicNamespaceLocation []string `xml:"tev:TopicNamespaceLocation,omitempty"`
	FixedTopicSet          bool     `xml:"wsnt:FixedTopicSet"`
	TopicSet               *struct {
		Any string `xml:",any"`
	} `xml:"tev:TopicSet,omitempty"`
}

type createPullPointSubscriptionResponse struct {
	XMLName               xml.Name              `xml:"tev:CreatePullPointSubscriptionResponse"`
	SubscriptionReference subscriptionReference `xml:"tev:SubscriptionReference"`
	CurrentTime           string                `xml:"wsnt:CurrentTime,omitempty"`
	TerminationTime       string                `xml:"wsnt:TerminationTime,omitempty"`
}
