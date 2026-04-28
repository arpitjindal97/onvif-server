package onvif

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/aragarwal/onvif-server/internal/logger"
)

// WS-Security structures.
type security struct {
	WSSE          string        `xml:"xmlns:wsse,attr"`
	WSU           string        `xml:"xmlns:wsu,attr"`
	UsernameToken usernameToken `xml:"wsse:UsernameToken"`
}

type usernameToken struct {
	Username string   `xml:"wsse:Username"`
	Password password `xml:"wsse:Password"`
	Nonce    nonce    `xml:"wsse:Nonce"`
	Created  string   `xml:"wsu:Created"`
}

type password struct {
	Type  string `xml:"Type,attr"`
	Value string `xml:",chardata"`
}

type nonce struct {
	EncodingType string `xml:"EncodingType,attr"`
	Value        string `xml:",chardata"`
}

// SOAP envelope structures used for marshalling responses.
type soapEnvelope struct {
	XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
	SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
	Header  *soapHeader `xml:"SOAP-ENV:Header,omitempty"`
	Body    interface{} `xml:"SOAP-ENV:Body"`
}

type soapHeader struct {
	Security *security `xml:"wsse:Security,omitempty"`
}

type soapFault struct {
	XMLName xml.Name `xml:"SOAP-ENV:Fault"`
	Code    struct {
		Value   string `xml:"SOAP-ENV:Value"`
		Subcode *struct {
			Value string `xml:"SOAP-ENV:Value"`
		} `xml:"SOAP-ENV:Subcode,omitempty"`
	} `xml:"SOAP-ENV:Code"`
	Reason struct {
		Text string `xml:"SOAP-ENV:Text"`
	} `xml:"SOAP-ENV:Reason"`
	Detail *struct {
		Text string `xml:",chardata"`
	} `xml:"SOAP-ENV:Detail,omitempty"`
}

// sendSOAPResponse marshals response inside a SOAP envelope and writes it.
func (s *Server) sendSOAPResponse(w http.ResponseWriter, response interface{}) {
	body := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Body"`
		Content interface{} `xml:",any"`
	}{
		Content: response,
	}

	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		TT      string      `xml:"xmlns:tt,attr"`
		TDS     string      `xml:"xmlns:tds,attr"`
		TRT     string      `xml:"xmlns:trt,attr"`
		TEV     string      `xml:"xmlns:tev,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		TT:      "http://www.onvif.org/ver10/schema",
		TDS:     "http://www.onvif.org/ver10/device/wsdl",
		TRT:     "http://www.onvif.org/ver10/media/wsdl",
		TEV:     "http://www.onvif.org/ver10/events/wsdl",
		Body:    body,
	}

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

// sendSOAPFault writes a SOAP fault envelope.
func (s *Server) sendSOAPFault(w http.ResponseWriter, code, subcode, reason string) {
	fault := soapFault{}
	fault.Code.Value = code
	if subcode != "" {
		fault.Code.Subcode = &struct {
			Value string `xml:"SOAP-ENV:Value"`
		}{Value: subcode}
	}
	fault.Reason.Text = reason

	body := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Body"`
		Content interface{} `xml:",any"`
	}{
		Content: fault,
	}

	envelope := struct {
		XMLName xml.Name    `xml:"SOAP-ENV:Envelope"`
		SOAPENV string      `xml:"xmlns:SOAP-ENV,attr"`
		TER     string      `xml:"xmlns:ter,attr"`
		Body    interface{} `xml:"SOAP-ENV:Body"`
	}{
		SOAPENV: "http://www.w3.org/2003/05/soap-envelope",
		TER:     "http://www.onvif.org/ver10/error",
		Body:    body,
	}

	output, _ := xml.MarshalIndent(envelope, "", "  ")

	logger.Debug("[%s] Sending SOAP Fault: %s - %s", s.config.Name, code, reason)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(xml.Header))
	w.Write(output)
}

// validateSecurity verifies the WS-Security UsernameToken/PasswordDigest header.
func (s *Server) validateSecurity(sec *security) bool {
	if sec == nil {
		return true
	}

	username := sec.UsernameToken.Username
	pw := sec.UsernameToken.Password.Value
	n := sec.UsernameToken.Nonce.Value
	created := sec.UsernameToken.Created

	logger.Debug("[%s] Auth check - Username: '%s', Nonce: '%s', Created: '%s', ReceivedDigest: '%s'",
		s.config.Name, username, n, created, pw)

	if username != s.username {
		logger.Debug("[%s] Username mismatch: got '%s', expected '%s'", s.config.Name, username, s.username)
		return false
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(n)
	if err != nil {
		logger.Debug("[%s] Failed to decode nonce: %v", s.config.Name, err)
		return false
	}

	// PasswordDigest = Base64( SHA-1( nonce + created + password ) )
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(s.password))
	expectedDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	logger.Debug("[%s] Expected digest: '%s', Received digest: '%s'", s.config.Name, expectedDigest, pw)

	return pw == expectedDigest
}

// generateNonce returns a base64-encoded random nonce. Currently unused but
// retained as it accompanies the password-digest helpers.
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// generatePasswordDigest computes the WS-Security password digest.
func generatePasswordDigest(nonceB64, created, pw string) string {
	nonceBytes, _ := base64.StdEncoding.DecodeString(nonceB64)
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(pw))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// calculateMD5 returns the hex MD5 hash of the input string.
func calculateMD5(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum(nil))
}
