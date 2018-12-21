package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	gst "github.com/pions/webrtc/examples/util/gstreamer-src"
	"github.com/pions/webrtc/pkg/ice"
)

func main() {
	addr := flag.String("address", ":50000", "Address that the HTTP server is hosted on.")
	flag.Parse()
	/*go func() {
		cmd := exec.Command("go", "run", "answer/main.go")
		cmdOutput := &bytes.Buffer{}
		cmd.Stdout = cmdOutput
		err := cmd.Run()
		if err != nil {
			os.Stderr.WriteString(err.Error())
		}
		fmt.Print(string(cmdOutput.Bytes()))
	}()

	time.Sleep(time.Second * 10)
	*/
	webrtc.RegisterDefaultCodecs()

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(config)
	util.Check(err)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	Track, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
	util.Check(err)

	_, err = peerConnection.AddTrack(Track)

	offer, err := peerConnection.CreateOffer(nil)
	util.Check(err)

	answer := mustSignalViaHTTP(offer, *addr)

	err = peerConnection.SetRemoteDescription(answer)
	util.Check(err)

	gst.CreatePipeline(webrtc.VP8, Track.Samples).Start()

	select {}
}

// mustSignalViaHTTP exchange the SDP offer and answer using an HTTP Post request.
func mustSignalViaHTTP(offer webrtc.RTCSessionDescription, address string) webrtc.RTCSessionDescription {
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(offer)
	util.Check(err)

	resp, err := http.Post("http://"+address, "application/json; charset=utf-8", b)
	util.Check(err)
	defer resp.Body.Close()

	var answer webrtc.RTCSessionDescription
	err = json.NewDecoder(resp.Body).Decode(&answer)
	util.Check(err)

	return answer
}
