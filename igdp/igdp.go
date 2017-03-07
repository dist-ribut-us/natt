// Package igdp implements then Internet Gateway Device Protocol to handle
// communication with a uPNP device to open an external port for NAT Traversal.
package igdp

import (
	"encoding/xml"
	"fmt"
	"github.com/dist-ribut-us/rnet"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

// this allows errors to be defined as const instead of var
type defineErr string

func (d defineErr) Error() string {
	return string(d)
}

// LocalIP address
var LocalIP string

// URLBase IP and port for communication with uPNP device
var URLBase string

// ControlURL is the url on URLBase to send requests
var ControlURL string

// Control protocol to use in requests
var Control string

// Location to request device description
var Location string

// ErrNoLocalIP error will occure when a local IP cannot be determined
const ErrNoLocalIP = defineErr("No local IP")

const searchMessage = "M-SEARCH * HTTP/1.1\r\n" +
	"HOST: 239.255.255.250:1900\r\n" +
	"ST: urn:schemas-upnp-org:service:WANIPConnection:1\r\n" +
	"MAN: \"ssdp:discover\"\r\n" +
	"MX: 3\r\n" +
	"\r\n"

// Setup will populate LocalIP, URLBase, ControlURL, Control and Location. It
// must be called prior to calling any other functions in this package.
func Setup() error {
	if LocalIP == "" {
		if localIPs := rnet.GetLocalIPs(); len(localIPs) > 0 {
			LocalIP = localIPs[0]
		} else {
			return ErrNoLocalIP
		}
	}

	remotAddr, err := net.ResolveUDPAddr("udp", "239.255.255.250:1900")
	if err != nil {
		return err
	}
	localAddr, err := net.ResolveUDPAddr("udp", LocalIP+":0")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte(searchMessage), remotAddr)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024)

	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return err
	}

	result := string(buf[:n])
	lines := strings.Split(result, "\r\n")
	for _, line := range lines {
		nameValues := strings.SplitAfterN(line, ":", 2)
		if len(nameValues) < 2 {
			continue
		}
		name := strings.ToUpper(strings.Trim(nameValues[0], " :"))
		val := strings.TrimSpace(nameValues[1])

		switch name {
		case "LOCATION":
			Location = val
		}
	}
	return getDeviceDescription()
}

type service struct {
	XMLName     xml.Name `xml:"service"`
	Type        string   `xml:"serviceType"`
	ID          string   `xml:"serviceId"`
	ControlURL  string   `xml:"controlURL"`
	EventSubURL string   `xml:"eventSubURL"`
	SCPDURL     string   `xml:"SCPDURL"`
}

func getDeviceDescription() error {
	req, err := http.NewRequest("GET", Location, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Connection", "keep-alive")

	decoder, err := xmlResponse(req)
	if err != nil {
		return err
	}
	iter := newXMLIter(decoder)
	for iter.next() {
		if iter.t.Name.Local == "service" {
			s := &service{}
			decoder.DecodeElement(s, iter.t)
			if strings.HasSuffix(s.Type, "Connection:1") {
				ControlURL = s.ControlURL
				Control = s.Type
				break
			}
		} else if iter.t.Name.Local == "URLBase" {
			t, _ := decoder.Token()
			if c, ok := t.(xml.CharData); ok {
				URLBase = strings.Trim(string(c), "/")
			}
		}
	}

	return iter.err
}

const soapEnv = `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<SOAP-ENV:Envelope
 SOAP-ENV:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"
 xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"
 xmlns:SOAP-ENC="http://schemas.xmlsoap.org/soap/encoding/"
 xmlns:xsi="http://www.w3.org/1999/XMLSchema-instance"
 xmlns:xsd="http://www.w3.org/1999/XMLSchema">
<SOAP-ENV:Body>
  %s
</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

// ExternalIP that the uPNP device presents to the rest of the internet.
// Populated by calling GetExternalIP
var ExternalIP string

// GetExternalIP will populate ExternalIP. The value is also returned, along
// with an error.
func GetExternalIP() (string, error) {
	if Control == "" {
		resp, err := http.Get("http://myexternalip.com/raw")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		return strings.TrimSpace(string(b)), err
	}

	bodyStr := fmt.Sprintf(`<m:GetExternalIPAddress xmlns:m="%s"></m:GetExternalIPAddress>`, Control)
	bodyStr = fmt.Sprintf(soapEnv, bodyStr)
	req, err := http.NewRequest("POST", URLBase+ControlURL, strings.NewReader(bodyStr))
	if err != nil {
		return "", err
	}
	req.Header.Set("SOAPAction", `"`+Control+`#GetExternalIPAddress"`)
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Connection", "Close")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyStr)))

	decoder, err := xmlResponse(req)
	if err != nil {
		return "", err
	}
	iter := newXMLIter(decoder)
	for iter.next() {
		if iter.t.Name.Local == "NewExternalIPAddress" {
			t, _ := decoder.Token()
			if c, ok := t.(xml.CharData); ok {
				ExternalIP = string(c)
				return strings.TrimSpace(ExternalIP), nil
			}
			break
		}
	}
	return "", iter.err
}

const bodyStr = `<m:AddPortMapping xmlns:m="urn:schemas-upnp-org:service:WANPPPConnection:1">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>UDP</NewProtocol>
<NewInternalPort>%d</NewInternalPort>
<NewInternalClient>%s</NewInternalClient>
<NewEnabled>1</NewEnabled>
<NewPortMappingDescription>upnpbind</NewPortMappingDescription>
<NewLeaseDuration>0</NewLeaseDuration>
</m:AddPortMapping>`

// AddPortMapping maps a local port to a port on the ExternalIP.
func AddPortMapping(localPort, remotePort int) (string, error) {
	body := fmt.Sprintf(bodyStr, remotePort, localPort, LocalIP)
	body = fmt.Sprintf(soapEnv, body)
	req, err := http.NewRequest("POST", URLBase+ControlURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("SOAPAction", `"`+Control+`#AddPortMapping"`)
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Connection", "Close")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	r, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return string(r), defineErr(resp.Status)
	}

	return string(r), nil
}

// xmlResponse takes an http Request that expects xml in the body and returns an
// xml Decoder that handles the response body.
func xmlResponse(req *http.Request) (*xml.Decoder, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, defineErr(resp.Status)
	}
	return xml.NewDecoder(resp.Body), nil
}

// xmlIter iterates over the starting tags in an xml document
type xmlIter struct {
	d   *xml.Decoder
	t   *xml.StartElement
	err error
}

func newXMLIter(d *xml.Decoder) *xmlIter {
	return &xmlIter{d: d}
}

// sets
func (x *xmlIter) next() bool {
	for {
		t, err := x.d.Token()
		if err != nil || t == nil {
			x.t = nil
			x.err = err
			break
		}
		if st, ok := t.(xml.StartElement); ok {
			x.t = &st
			break
		}
	}
	return x.t != nil && x.err == nil
}
