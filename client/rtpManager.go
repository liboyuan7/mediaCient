package client

import (
	"fmt"
	"github.com/appcrash/GoRTP/rtp"
	"github.com/streamFunc/RTPGoAPI/jrtp"
	"net"
	"time"
)

type RTPSession interface {
	NewDataPacket(stamp uint32)
	AddRemote(remoteIpStr string, remotePort int) (index uint32, err error)
	CreateDataReceiveChan()
	RemoveDataReceiveChan()
	CreateCtrlEventChan()
	RemoveCtrlEventChan()
	WriteData() (k int, err error)
	SsrcStreamOutForIndex(streamIndex uint32)
	NewSsrcStreamOut(ip *net.IPAddr, port int, ssrc uint32, sequenceNo uint16) (index uint32)
	StartSession() error
	CloseSession()
}

type GoRtpSession struct {
	name            string
	session         *rtp.Session
	packet          *rtp.DataPacket
	dataReceiveChan chan *rtp.DataPacket
	ctrlEventChan   rtp.CtrlEventChan
	streamsOut      map[uint32]*rtp.SsrcStream
}
type jRtpSession struct {
	name            string
	session         *jrtp.Session
	packet          *jrtp.DataPacket
	dataReceiveChan chan *jrtp.DataPacket
	ctrlEventChan   jrtp.CtrlEventChan
	streamsOut      map[uint32]*jrtp.SsrcStream
}

func (s *GoRtpSession) NewDataPacket(stamp uint32) {
	if s.session != nil {
		s.packet = s.session.NewDataPacket(stamp)
	}
	return
}

func (s *GoRtpSession) AddRemote(remoteIpStr string, remotePort int) (index uint32, err error) {
	if s.session != nil {
		remoteIp, _ := net.ResolveIPAddr("ip", remoteIpStr)
		remote := &rtp.Address{
			IPAddr:   remoteIp.IP,
			DataPort: remotePort,
			CtrlPort: 1 + remotePort,
			Zone:     "",
		}
		return s.session.AddRemote(remote)
	}
	return 0, nil
}

func (s *GoRtpSession) CreateDataReceiveChan() {
	if s.session != nil {
		s.dataReceiveChan = s.session.CreateDataReceiveChan()
	}
}

func (s *GoRtpSession) RemoveDataReceiveChan() {
	if s.session != nil {
		s.session.RemoveDataReceiveChan()
	}
}

func (s *GoRtpSession) CreateCtrlEventChan() {
	if s.session != nil {
		s.ctrlEventChan = s.session.CreateCtrlEventChan()
	}
}

func (s *GoRtpSession) RemoveCtrlEventChan() {
	if s.session != nil {
		s.session.RemoveCtrlEventChan()
	}
}

func (s *GoRtpSession) SsrcStreamOutForIndex(streamIndex uint32) {
	if s.session != nil {
		s.streamsOut[streamIndex] = s.session.SsrcStreamOutForIndex(streamIndex)
	}
}

func (s *GoRtpSession) NewSsrcStreamOut(ip *net.IPAddr, port int, ssrc uint32, sequenceNo uint16) (index uint32) {
	if s.session != nil {
		index, _ = s.session.NewSsrcStreamOut(&rtp.Address{
			IPAddr:   ip.IP,
			DataPort: port,
			CtrlPort: 1 + port,
			Zone:     "",
		}, 0, 0)
	}
	return index
}

func (s *GoRtpSession) StartSession() error {
	if s.session != nil {
		return s.session.StartSession()
	}
	return nil
}

func (s *GoRtpSession) WriteData() (k int, err error) {
	if s.session != nil {
		return s.session.WriteData(s.packet)
	}
	return 0, nil
}

func (s *GoRtpSession) CloseSession() {
	if s.session != nil {
		s.session.CloseSession()
	}
}

func (js *jRtpSession) NewDataPacket(stamp uint32) {
	if js.session != nil {
		js.packet = js.session.NewDataPacket(stamp)
	}
}

func (js *jRtpSession) AddRemote(remoteIpStr string, remotePort int) (index uint32, err error) {
	if js.session != nil {
		remoteIp, _ := net.ResolveIPAddr("ip", remoteIpStr)
		remote := &jrtp.Address{
			IPAddr:   remoteIp.IP,
			DataPort: remotePort,
			CtrlPort: 1 + remotePort,
			Zone:     "",
		}
		return js.session.AddRemote(remote)
	}
	return 0, nil
}

func (js *jRtpSession) CreateDataReceiveChan() {
	if js.session != nil {
		js.dataReceiveChan = js.session.CreateDataReceiveChan()
	}
}

func (js *jRtpSession) RemoveDataReceiveChan() {
	if js.session != nil {
		js.session.RemoveDataReceiveChan()
	}
}

func (js *jRtpSession) CreateCtrlEventChan() {
	if js.session != nil {
		js.ctrlEventChan = js.session.CreateCtrlEventChan()
	}
}

func (js *jRtpSession) RemoveCtrlEventChan() {
	if js.session != nil {
		js.session.RemoveCtrlEventChan()
	}
}

func (js *jRtpSession) WriteData() (k int, err error) {
	if js.session != nil {
		return js.session.WriteData(js.packet)
	}
	return 0, nil
}

func (js *jRtpSession) SsrcStreamOutForIndex(streamIndex uint32) {
	if js.session != nil {
		js.streamsOut[streamIndex] = js.session.SsrcStreamOutForIndex(streamIndex)
	}
}

func (js *jRtpSession) NewSsrcStreamOut(ip *net.IPAddr, port int, ssrc uint32, sequenceNo uint16) (index uint32) {
	if js.session != nil {
		index, _ = js.session.NewSsrcStreamOut(&jrtp.Address{
			IPAddr:   ip.IP,
			DataPort: port,
			CtrlPort: 1 + port,
			Zone:     "",
		}, 0, 0)
	}
	return index
}

func (js *jRtpSession) StartSession() error {
	if js.session != nil {
		return js.session.StartSession()
	}
	return nil
}

func (js *jRtpSession) CloseSession() {
	if js.session != nil {
		js.session.CloseSession()
	}
	return
}

func NewSession(rtpMode string, addr *net.IPAddr, localPort int) RTPSession {
	if rtpMode == "goRtp" {
		tpLocal, _ := rtp.NewTransportUDP(addr, localPort, "")
		return &GoRtpSession{
			name:    "goRtp",
			session: rtp.NewSession(tpLocal, tpLocal),
		}
	} else if rtpMode == "jRtp" {
		tpLocal, _ := jrtp.NewTransportUDP(addr, localPort, "")
		return &jRtpSession{
			name:    "jRtp",
			session: jrtp.NewSession(tpLocal, tpLocal),
		}
	} else {
		tpLocal, _ := rtp.NewTransportUDP(addr, localPort, "")
		return &GoRtpSession{
			name:    "goRtp",
			session: rtp.NewSession(tpLocal, tpLocal),
		}
	}
}

func TestRtp() {
	fmt.Printf("test it...\n")
	localIp, _ := net.ResolveIPAddr("ip", "127.0.0.1")
	session := NewSession("jRtp", localIp, 7070)
	strIndex := session.NewSsrcStreamOut(localIp, 7070, 0, 0)

	if goRtpSession, ok := session.(*jRtpSession); ok {
		fmt.Println("Session name:", goRtpSession.name)

		// 访问 GoRtpSession 结构体的字段
		goRtpSession.session.SsrcStreamOutForIndex(strIndex).SetProfile("PCMA", 8)

		session.AddRemote("127.0.0.1", 7071)
		session.StartSession()

		goRtpSession.packet = goRtpSession.session.NewDataPacket(0)
		goRtpSession.packet.SetPayloadType(8)
		goRtpSession.packet.SetPayload(nil)
		session.WriteData()

	} else {
		fmt.Println("Session is not a jRtpSession")
	}

	time.Sleep(time.Second * 5)
	session.CloseSession()

}
