package onvif

import "encoding/xml"

// Media service types.
type getProfilesResponse struct {
	XMLName  xml.Name  `xml:"trt:GetProfilesResponse"`
	Profiles []profile `xml:"trt:Profiles"`
}

type profile struct {
	Token                       string                       `xml:"token,attr"`
	Fixed                       bool                         `xml:"fixed,attr"`
	Name                        string                       `xml:"tt:Name"`
	VideoSourceConfiguration    *videoSourceConfiguration    `xml:"tt:VideoSourceConfiguration,omitempty"`
	AudioSourceConfiguration    *audioSourceConfiguration    `xml:"tt:AudioSourceConfiguration,omitempty"`
	VideoEncoderConfiguration   *videoEncoderConfiguration   `xml:"tt:VideoEncoderConfiguration,omitempty"`
	AudioEncoderConfiguration   *audioEncoderConfiguration   `xml:"tt:AudioEncoderConfiguration,omitempty"`
	VideoAnalyticsConfiguration *videoAnalyticsConfiguration `xml:"tt:VideoAnalyticsConfiguration,omitempty"`
	PTZConfiguration            *ptzConfiguration            `xml:"tt:PTZConfiguration,omitempty"`
	MetadataConfiguration       *metadataConfiguration       `xml:"tt:MetadataConfiguration,omitempty"`
}

type videoSourceConfiguration struct {
	Token       string `xml:"token,attr"`
	Name        string `xml:"tt:Name"`
	UseCount    int    `xml:"tt:UseCount"`
	SourceToken string `xml:"tt:SourceToken"`
	Bounds      bounds `xml:"tt:Bounds"`
}

type bounds struct {
	X      int `xml:"x,attr"`
	Y      int `xml:"y,attr"`
	Width  int `xml:"width,attr"`
	Height int `xml:"height,attr"`
}

type videoEncoderConfiguration struct {
	Token          string       `xml:"token,attr"`
	Name           string       `xml:"tt:Name"`
	UseCount       int          `xml:"tt:UseCount"`
	Encoding       string       `xml:"tt:Encoding"`
	Resolution     resolution   `xml:"tt:Resolution"`
	Quality        float64      `xml:"tt:Quality"`
	RateControl    *rateControl `xml:"tt:RateControl,omitempty"`
	H264           *h264Config  `xml:"tt:H264,omitempty"`
	Multicast      multicast    `xml:"tt:Multicast"`
	SessionTimeout string       `xml:"tt:SessionTimeout"`
}

type resolution struct {
	Width  int `xml:"tt:Width"`
	Height int `xml:"tt:Height"`
}

type rateControl struct {
	FrameRateLimit   int `xml:"tt:FrameRateLimit"`
	EncodingInterval int `xml:"tt:EncodingInterval"`
	BitrateLimit     int `xml:"tt:BitrateLimit"`
}

type h264Config struct {
	GovLength   int    `xml:"tt:GovLength"`
	H264Profile string `xml:"tt:H264Profile"`
}

type multicast struct {
	Address   multicastAddress `xml:"tt:Address"`
	Port      int              `xml:"tt:Port"`
	TTL       int              `xml:"tt:TTL"`
	AutoStart bool             `xml:"tt:AutoStart"`
}

type multicastAddress struct {
	Type        string `xml:"tt:Type"`
	IPv4Address string `xml:"tt:IPv4Address,omitempty"`
	IPv6Address string `xml:"tt:IPv6Address,omitempty"`
}

type audioSourceConfiguration struct {
	Token       string `xml:"token,attr"`
	Name        string `xml:"tt:Name"`
	UseCount    int    `xml:"tt:UseCount"`
	SourceToken string `xml:"tt:SourceToken"`
}

type audioEncoderConfiguration struct {
	Token          string    `xml:"token,attr"`
	Name           string    `xml:"tt:Name"`
	UseCount       int       `xml:"tt:UseCount"`
	Encoding       string    `xml:"tt:Encoding"`
	Bitrate        int       `xml:"tt:Bitrate"`
	SampleRate     int       `xml:"tt:SampleRate"`
	Multicast      multicast `xml:"tt:Multicast"`
	SessionTimeout string    `xml:"tt:SessionTimeout"`
}

type videoAnalyticsConfiguration struct {
	Token                        string   `xml:"token,attr"`
	Name                         string   `xml:"tt:Name"`
	UseCount                     int      `xml:"tt:UseCount"`
	AnalyticsEngineConfiguration struct{} `xml:"tt:AnalyticsEngineConfiguration"`
	RuleEngineConfiguration      struct{} `xml:"tt:RuleEngineConfiguration"`
}

type ptzConfiguration struct {
	Token     string `xml:"token,attr"`
	Name      string `xml:"tt:Name"`
	UseCount  int    `xml:"tt:UseCount"`
	NodeToken string `xml:"tt:NodeToken"`
}

type metadataConfiguration struct {
	Token          string `xml:"token,attr"`
	Name           string `xml:"tt:Name"`
	UseCount       int    `xml:"tt:UseCount"`
	SessionTimeout string `xml:"tt:SessionTimeout"`
}

type getStreamUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetStreamUriResponse"`
	MediaUri mediaUri `xml:"trt:MediaUri"`
}

type mediaUri struct {
	Uri                 string `xml:"tt:Uri"`
	InvalidAfterConnect bool   `xml:"tt:InvalidAfterConnect"`
	InvalidAfterReboot  bool   `xml:"tt:InvalidAfterReboot"`
	Timeout             string `xml:"tt:Timeout"`
}

type getSnapshotUriResponse struct {
	XMLName  xml.Name `xml:"trt:GetSnapshotUriResponse"`
	MediaUri mediaUri `xml:"trt:MediaUri"`
}

type getVideoSourcesResponse struct {
	XMLName      xml.Name      `xml:"trt:GetVideoSourcesResponse"`
	VideoSources []videoSource `xml:"trt:VideoSources"`
}

type videoSource struct {
	Token      string     `xml:"token,attr"`
	Framerate  float64    `xml:"tt:Framerate"`
	Resolution resolution `xml:"tt:Resolution"`
	Imaging    *struct{}  `xml:"tt:Imaging,omitempty"`
}

type getAudioSourcesResponse struct {
	XMLName      xml.Name      `xml:"trt:GetAudioSourcesResponse"`
	AudioSources []audioSource `xml:"trt:AudioSources"`
}

type audioSource struct {
	Token    string `xml:"token,attr"`
	Channels int    `xml:"tt:Channels"`
}

type getVideoEncoderConfigurationsResponse struct {
	XMLName        xml.Name                    `xml:"trt:GetVideoEncoderConfigurationsResponse"`
	Configurations []videoEncoderConfiguration `xml:"trt:Configurations"`
}

type getVideoEncoderConfigurationResponse struct {
	XMLName       xml.Name                  `xml:"trt:GetVideoEncoderConfigurationResponse"`
	Configuration videoEncoderConfiguration `xml:"trt:Configuration"`
}

type getVideoEncoderConfigurationOptionsResponse struct {
	XMLName xml.Name                         `xml:"trt:GetVideoEncoderConfigurationOptionsResponse"`
	Options videoEncoderConfigurationOptions `xml:"trt:Options"`
}

type videoEncoderConfigurationOptions struct {
	QualityRange qualityRange      `xml:"tt:QualityRange"`
	JPEG         *jpegOptions      `xml:"tt:JPEG,omitempty"`
	MPEG4        *mpeg4Options     `xml:"tt:MPEG4,omitempty"`
	H264         *h264Options      `xml:"tt:H264,omitempty"`
	Extension    *encoderExtension `xml:"tt:Extension,omitempty"`
}

type qualityRange struct {
	Min int `xml:"tt:Min"`
	Max int `xml:"tt:Max"`
}

type jpegOptions struct {
	ResolutionsAvailable  []resolution `xml:"tt:ResolutionsAvailable"`
	FrameRateRange        intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange intRange     `xml:"tt:EncodingIntervalRange"`
}

type mpeg4Options struct {
	ResolutionsAvailable   []resolution `xml:"tt:ResolutionsAvailable"`
	GovLengthRange         intRange     `xml:"tt:GovLengthRange"`
	FrameRateRange         intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange  intRange     `xml:"tt:EncodingIntervalRange"`
	Mpeg4ProfilesSupported []string     `xml:"tt:Mpeg4ProfilesSupported"`
}

type h264Options struct {
	ResolutionsAvailable  []resolution `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        intRange     `xml:"tt:GovLengthRange"`
	FrameRateRange        intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange intRange     `xml:"tt:EncodingIntervalRange"`
	H264ProfilesSupported []string     `xml:"tt:H264ProfilesSupported"`
}

type intRange struct {
	Min int `xml:"tt:Min"`
	Max int `xml:"tt:Max"`
}

type encoderExtension struct {
	JPEG      *jpegOptions2  `xml:"tt:JPEG,omitempty"`
	MPEG4     *mpeg4Options2 `xml:"tt:MPEG4,omitempty"`
	H264      *h264Options2  `xml:"tt:H264,omitempty"`
	Extension *struct{}      `xml:"tt:Extension,omitempty"`
}

type jpegOptions2 struct {
	ResolutionsAvailable  []resolution `xml:"tt:ResolutionsAvailable"`
	FrameRateRange        intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange intRange     `xml:"tt:EncodingIntervalRange"`
	BitrateRange          intRange     `xml:"tt:BitrateRange"`
}

type mpeg4Options2 struct {
	ResolutionsAvailable   []resolution `xml:"tt:ResolutionsAvailable"`
	GovLengthRange         intRange     `xml:"tt:GovLengthRange"`
	FrameRateRange         intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange  intRange     `xml:"tt:EncodingIntervalRange"`
	Mpeg4ProfilesSupported []string     `xml:"tt:Mpeg4ProfilesSupported"`
	BitrateRange           intRange     `xml:"tt:BitrateRange"`
}

type h264Options2 struct {
	ResolutionsAvailable  []resolution `xml:"tt:ResolutionsAvailable"`
	GovLengthRange        intRange     `xml:"tt:GovLengthRange"`
	FrameRateRange        intRange     `xml:"tt:FrameRateRange"`
	EncodingIntervalRange intRange     `xml:"tt:EncodingIntervalRange"`
	H264ProfilesSupported []string     `xml:"tt:H264ProfilesSupported"`
	BitrateRange          intRange     `xml:"tt:BitrateRange"`
}
