package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	gst "github.com/pions/webrtc/examples/util/gstreamer-sink"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcp"
)

func main() {
	addr := flag.String("address", ":50000", "Address to host the HTTP server on.")
	flag.Parse()

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
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	peerConnection.OnTrack(func(track *webrtc.RTCTrack) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				err := peerConnection.SendRTCP(&rtcp.PictureLossIndication{MediaSSRC: track.Ssrc})
				if err != nil {
					fmt.Println(err)
				}
			}
		}()

		codec := track.Codec
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType, codec.Name)
		pipeline := gst.CreatePipeline(codec.Name)
		pipeline.Start()
		for {
			p := <-track.Packets
			pipeline.Push(p.Raw)
		}
	})

	// Exchange the offer/answer via HTTP
	offerChan, answerChan := mustSignalViaHTTP(*addr)

	// Wait for the remote SessionDescription
	offer := <-offerChan

	err = peerConnection.SetRemoteDescription(offer)
	util.Check(err)

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	util.Check(err)

	// Send the answer
	answerChan <- answer

	// Block forever
	select {}
}

// mustSignalViaHTTP exchange the SDP offer and answer using an HTTP server.
func mustSignalViaHTTP(address string) (offerOut chan webrtc.RTCSessionDescription, answerIn chan webrtc.RTCSessionDescription) {
	offerOut = make(chan webrtc.RTCSessionDescription)
	answerIn = make(chan webrtc.RTCSessionDescription)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var offer webrtc.RTCSessionDescription
		err := json.NewDecoder(r.Body).Decode(&offer)
		util.Check(err)

		offerOut <- offer
		answer := <-answerIn

		err = json.NewEncoder(w).Encode(answer)
		util.Check(err)

	})

	go http.ListenAndServe(address, nil)
	fmt.Println("Listening on", address)

	return
}
