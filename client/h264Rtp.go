package client

import (
	"encoding/binary"
	"fmt"
	"github.com/appcrash/GoRTP/rtp"
	//"github.com/streamFunc/RTPGoAPI/rtp"
)

const (
	// The NAL unit type octet has the following format:
	//+---------------+
	//|0|1|2|3|4|5|6|7|
	//+-+-+-+-+-+-+-+-+
	//|F|NRI| TypeId    |
	//+---------------+

	HCNalTypeStapa uint8 = 24
	HCNalTypeFua   uint8 = 28
	HCNalTypeSei   uint8 = 6
	HCNalTypeSps   uint8 = 7
	HCNalTypePps   uint8 = 8
	HCNalTypeAu    uint8 = 9 // Access Unit Delimiter

	HCBitmaskNalType uint8 = 0x1f
	HCBitmaskRefIdc  uint8 = 0x60
	HCBitmaskFuStart uint8 = 0x80
	HCBitmaskFuEnd   uint8 = 0x40

	HCDefaultMtu = 1400
)

// HCPacketListFromH264Mode
// mtu is for payload, not including ip,udp headers
// nal of type stapA would not be created if disableStap set true
func HCPacketListFromH264Mode(annexbPayload []byte, pts uint32, payloadType uint8, mtu int, disableStap bool) (pl *HCRtpPacketList) {
	// packetization-mode == 1
	// Only single NAL unit packets, STAP-As, and FU-As MAY be used in this mode.
	nals := HCExtractNals(annexbPayload)
	var bufferedNals [][]byte
	var rtpPayloadArray [][]byte
	bufferedSize := 1 // stapA header with 1 byte
	i := 0
	for i < len(nals) {
		nal := nals[i]
		size := len(nal)
		if disableStap {
			goto noStap
		}
		if size+2+bufferedSize <= mtu {
			// nal size with 2 bytes in stapA
			bufferedNals = append(bufferedNals, nal)
			bufferedSize += size + 2
			i++
			continue
		} else {
			// this nal can not be aggregated, just flush buffered nals if any
			if len(bufferedNals) > 0 {
				if len(bufferedNals) == 1 {
					// single nal, no aggregation
					rtpPayloadArray = append(rtpPayloadArray, bufferedNals[0])
				} else {
					stapA := makeStapA(bufferedNals...)
					rtpPayloadArray = append(rtpPayloadArray, stapA)
				}
				bufferedNals = nil
				bufferedSize = 1
			}
		}

	noStap:
		// check this nal again
		if size > mtu {
			rtpPayload := makeFuA(mtu, nal)
			rtpPayloadArray = append(rtpPayloadArray, rtpPayload...)
			i++
		} else if !disableStap && size+2+bufferedSize < mtu {
			// size < mtu - (2 + bufferedSize)
			// if this nal can be put into stapA after buffer flushed, do it again
			continue
		} else {
			// mtu - (2 + bufferedSize) <= size <= mtu
			// rare case, just send as it is
			rtpPayloadArray = append(rtpPayloadArray, nal)
			i++
		}

	}

	// check if buffered nals exist for the last time
	if !disableStap && len(bufferedNals) > 0 {
		if len(bufferedNals) == 1 {
			// single nal, no aggregation
			rtpPayloadArray = append(rtpPayloadArray, bufferedNals[0])
		} else {
			stapA := makeStapA(bufferedNals...)
			rtpPayloadArray = append(rtpPayloadArray, stapA)
		}
	}

	// all payloads are in order, make the packet list
	makePacketList(&pl, rtpPayloadArray, pts, payloadType)
	return
}

func makePacketList(pl **HCRtpPacketList, rtpPayload [][]byte, pts uint32, payloadType uint8) {
	var packet *HCRtpPacketList
	prev := *pl
	for _, payload := range rtpPayload {
		packet = &HCRtpPacketList{
			Payload:     payload,
			Pts:         pts,
			PayloadType: payloadType,
		}
		if prev == nil {
			*pl = packet
		} else {
			prev.SetNext(packet)
		}
		prev = packet
	}
	if packet != nil {
		// set last packet mark bit if they are in the same access unit

		// TODO:
		//For aggregation packets (STAP and MTAP), the marker bit in the RTP
		//header MUST be set to the value that the marker bit of the last
		//NAL unit of the aggregation packet would have been if it were
		//transported in its own RTP packet
		packet.Marker = true
	}
}

// CExtractNals splits annexb payload by start code
func HCExtractNals(annexbPayload []byte) (nals [][]byte) {
	// start code can be of 4 bytes: 0x00,0x00,0x00,0x01 (sps,pps,first slice)
	// or 3 bytes: 0x00,0x00,0x01
	zeros := 0
	prevStart := 0
	totalLen := len(annexbPayload)
	for i, b := range annexbPayload {
		switch b {
		case 0x00:
			zeros++
			continue
		case 0x01:
			if zeros == 2 || zeros == 3 {
				// found a start code
				if i-zeros > prevStart {
					nal := annexbPayload[prevStart : i-zeros]
					nals = append(nals, nal)
				}
				prevStart = i + 1
				if prevStart >= totalLen {
					return
				}
			}
		}
		zeros = 0
	}
	if totalLen > prevStart {
		nals = append(nals, annexbPayload[prevStart:])
	}
	return
}

func HCPrintNal(nal []byte) {
	nalType := nal[0] & HCBitmaskNalType
	nalRefIdc := nal[0] & HCBitmaskRefIdc
	switch nalType {
	case HCNalTypeSps:
		fmt.Printf("type sps")
	case HCNalTypePps:
		fmt.Printf("type pps")
	case HCNalTypeSei:
		fmt.Printf("type sei")
	case HCNalTypeFua:
		fmt.Printf("type fua")
	case HCNalTypeStapa:
		fmt.Printf("type stapa")
	default:
		fmt.Printf("type: %d", nalType)
	}
	fmt.Printf("nal len is %d,refIdc is %d", len(nal), nalRefIdc)
}

func makeStapA(nals ...[]byte) (rtpPayload []byte) {
	// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                     RTP Header                                |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//| STAP-A NAL HDR|        NALU 1 Size            |  NALU 1 HDR   |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|             NALU 1 Data                                       |
	//:                                                               :
	//+               +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|               |     NALU 2 Size               | NALU 2 HDR    |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                        NALU 2 Data                            |
	//:                                                               :
	//|                               +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                               :...OPTIONAL RTP padding        |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	// The value of NRI MUST be the maximum of all the NAL units carried
	// in the aggregation packet.
	rtpPayload = []byte{0x00} // header placeholder, set it later
	size := make([]byte, 2)
	var maxNri uint8
	for _, nal := range nals {
		binary.BigEndian.PutUint16(size, uint16(len(nal)))
		rtpPayload = append(rtpPayload, size...)
		rtpPayload = append(rtpPayload, nal...)
		nri := nal[0] & HCBitmaskRefIdc
		if maxNri < nri {
			maxNri = nri
		}
	}
	rtpPayload[0] = maxNri | HCNalTypeStapa
	return
}

func makeFuA(mtu int, nal []byte) (rtpPayload [][]byte) {
	// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//| FU indicator |  FU header     |                               |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+                               |
	//|                                                               |
	//|                    FU payload                                 |
	//|                                                               |
	//|                               +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//|                               :...OPTIONAL RTP padding        |
	//+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	// The FU header has the following format:
	//+---------------+
	//|0|1|2|3|4|5|6|7|
	//+-+-+-+-+-+-+-+-+
	//|S|E|R| TypeId    |
	//+---------------+
	nri := nal[0] & HCBitmaskRefIdc
	nalType := nal[0] & HCBitmaskNalType
	indicator := nri | HCNalTypeFua
	isFirstFragment := true
	startPtr := 1 // skip the nal header
	remainingPayloadSize := len(nal) - startPtr
	maxPayloadSize := mtu - 2 // 2 = fu indicator + fu header

	for remainingPayloadSize > 0 {
		fragmentPayloadSize := remainingPayloadSize
		if fragmentPayloadSize > maxPayloadSize {
			fragmentPayloadSize = maxPayloadSize
		}
		payload := make([]byte, fragmentPayloadSize+2)
		payload[0] = indicator
		header := nalType
		if isFirstFragment {
			header |= HCBitmaskFuStart // set start bit
			isFirstFragment = false
		} else if fragmentPayloadSize == remainingPayloadSize {
			header |= HCBitmaskFuEnd // set end bit
		}
		payload[1] = header
		copy(payload[2:], nal[startPtr:startPtr+fragmentPayloadSize])
		rtpPayload = append(rtpPayload, payload)
		startPtr += fragmentPayloadSize
		remainingPayloadSize -= fragmentPayloadSize
	}
	return
}

// HCRtpPacketList is either received RTP data packet or generated packets by codecs that can be readily put to
// stack for transmission. audio data is usually one packet at a time as no pts is required, but video codecs can
// build multiple packets of the same pts. those packets can be linked and send to rtp stack as a whole.
type HCRtpPacketList struct {
	Payload     []byte // rtp payload
	RawBuffer   []byte // rtp payload + rtp header
	PayloadType uint8
	Pts         uint32 // presentation timestamp
	PrevPts     uint32 // previous packet's pts
	Marker      bool   // should mark-bit in rtp header be set?
	Ssrc        uint32
	Csrc        []uint32

	next *HCRtpPacketList // more RtpPacketList, if any
}

func MyNewPacketListFromRtpPacket(packet *rtp.DataPacket) *HCRtpPacketList {
	if packet.InUse() <= 0 || packet.Buffer() == nil {
		return nil
	}
	return &HCRtpPacketList{
		Payload:     packet.Payload(),
		RawBuffer:   packet.Buffer()[:packet.InUse()],
		PayloadType: packet.PayloadType(),
		Pts:         packet.Timestamp(),
		Marker:      packet.Marker(),
		Ssrc:        packet.Ssrc(),
		Csrc:        packet.CsrcList(),
	}
}

func (pl *HCRtpPacketList) Iterate(f func(p *HCRtpPacketList)) {
	ppl := pl
	for ppl != nil {
		f(ppl)
		ppl = ppl.next
	}
}

func (pl HCRtpPacketList) CloneSingle() *HCRtpPacketList {
	return &HCRtpPacketList{
		Payload:     pl.Payload,
		RawBuffer:   pl.RawBuffer,
		PayloadType: pl.PayloadType,
		Pts:         pl.Pts,
		Marker:      pl.Marker,
		Ssrc:        pl.Ssrc,
		Csrc:        pl.Csrc,
	}
}

func (pl *HCRtpPacketList) Clone() *HCRtpPacketList {
	var cloned, current *HCRtpPacketList
	pl.Iterate(func(packet *HCRtpPacketList) {
		newPacket := packet.CloneSingle()
		if cloned == nil {
			cloned = newPacket
		} else {
			current.next = newPacket
		}

		current = newPacket
	})
	return cloned
}

func (pl *HCRtpPacketList) Next() *HCRtpPacketList {
	return pl.next
}

func (pl *HCRtpPacketList) SetNext(npl *HCRtpPacketList) {
	pl.next = npl
}

func (pl *HCRtpPacketList) GetLast() *HCRtpPacketList {
	ppl := pl
	for ppl.next != nil {
		ppl = ppl.next
	}
	return ppl
}

func (pl *HCRtpPacketList) Len() (length int) {
	pl.Iterate(func(ppl *HCRtpPacketList) {
		length++
	})
	return
}
