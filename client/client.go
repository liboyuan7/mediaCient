package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/appcrash/media/server/rpc"
	"github.com/streamFunc/RTPGoAPI/rtp"
	//"github.com/appcrash/GoRTP/rtp"
	"github.com/streamFunc/mediaClient/port"
	"google.golang.org/grpc"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

var (
	GrpcIp   = "127.0.0.1"
	GrpcPort = 5678
)

type recvFunc func(event *rpc.SystemEvent)

type client struct {
	instanceId      string
	conn            *grpc.ClientConn
	mediaClient     rpc.MediaApiClient
	sysStream       rpc.MediaApi_SystemChannelClient
	frame           []byte
	h264PacketChan  chan *MyH264Packet
	audioPacketChan chan []byte
}

type MyH264Packet struct {
	Payload []byte
	Pts     int
}

func (c *client) keepalive(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 2)
	for {
		select {
		case <-ticker.C:
			c.sysStream.Send(&rpc.SystemEvent{
				Cmd:        rpc.SystemCommand_KEEPALIVE,
				InstanceId: c.instanceId,
			})
		case <-ctx.Done():
			return
		}
	}
}

func (c *client) connect(onReceive recvFunc) {
	var opts = []grpc.DialOption{grpc.WithInsecure()}
	var callOpts []grpc.CallOption
	conn, err1 := grpc.Dial(fmt.Sprintf("%v:%v", GrpcIp, GrpcPort), opts...)
	if err1 != nil {
		panic(err1)
	}
	c.conn = conn
	c.mediaClient = rpc.NewMediaApiClient(conn)

	// register myself first
	stream, err2 := c.mediaClient.SystemChannel(context.Background(), callOpts...)
	if err2 != nil {
		panic(err2)
	}
	stream.Send(&rpc.SystemEvent{
		Cmd:        rpc.SystemCommand_REGISTER,
		InstanceId: c.instanceId,
	})
	c.sysStream = stream

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				return
			}
			if err != nil {
				fmt.Printf("receive error: %v", err)
				return
			}
			onReceive(in)
		}
	}()
}

func (c *client) reportSessionInfo(ctx context.Context, sessionId string) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			c.sysStream.Send(&rpc.SystemEvent{
				Cmd:       rpc.SystemCommand_SESSION_INFO,
				SessionId: sessionId,
			})
		case <-ctx.Done():
			return
		}
	}
}

func (c *client) close() {
	c.conn.Close()
}

func StartSessionCall(p *port.MyPortPool, id string, isAudio bool, graphDesc string) {
	instanceId := id
	c := &client{instanceId: instanceId}
	c.h264PacketChan = make(chan *MyH264Packet, 32)
	c.audioPacketChan = make(chan []byte, 32)

	c.connect(func(event *rpc.SystemEvent) {
		//fmt.Printf("recv event: %v\n", event)
	})
	ctx, cancel := context.WithCancel(context.Background())

	var opts []grpc.CallOption
	var codecs []*rpc.CodecInfo
	peerPort := uint32(p.Get())

	if isAudio {
		codecs = []*rpc.CodecInfo{{
			PayloadNumber: 8,
			PayloadType:   rpc.CodecType_PCM_ALAW,
			CodecParam:    "",
		}}
	} else {
		codecs = []*rpc.CodecInfo{{
			PayloadNumber: 123, // Modify this according to your specific payload number for H.264
			PayloadType:   rpc.CodecType_H264,
			CodecParam:    "",
		}}
	}
	session, err := c.mediaClient.PrepareSession(ctx, &rpc.CreateParam{
		PeerIp:     "127.0.0.1",
		PeerPort:   peerPort,
		Codecs:     codecs,
		GraphDesc:  graphDesc,
		InstanceId: instanceId,
	}, opts...)
	if err != nil {
		//panic(err)
		fmt.Printf("error PrepareSession fail %v\n", err)
		return

	}

	localPort := uint32(p.Get())
	if _, err = c.mediaClient.UpdateSession(ctx, &rpc.UpdateParam{SessionId: session.SessionId, PeerPort: localPort}, opts...); err != nil {
		//panic(err)
		fmt.Printf("error UpdateSession fail %v\n", err)
		return
	}
	if _, err = c.mediaClient.StartSession(ctx, &rpc.StartParam{SessionId: session.SessionId}, opts...); err != nil {
		//panic(err)
		fmt.Printf("error StartSession fail %v\n", err)
		return
	}
	go c.keepalive(ctx)
	time.Sleep(1 * time.Second)

	var cancelRtp context.CancelFunc
	if cancelRtp, err = c.mockSendRtp(id, "127.0.0.1", int(localPort), session.LocalIp, int(session.LocalRtpPort), isAudio); err != nil {
		//panic(err)
		fmt.Printf("error mockSendRtp fail %v\n", err)
		return
	}
	go c.reportSessionInfo(ctx, session.SessionId)

	/*if _, err = c.mediaClient.ExecuteAction(ctx, &rpc.Action{
		SessionId: session.SessionId,
		Cmd:       "exec",
		CmdArg:    graphDesc + " <-- 'a b c d'",
	}, opts...); err != nil {
		panic(err)
	}*/
	if isAudio {
		//go c.getAudioData("./audio.pcm")
		go c.getAudioData1()
	} else {
		//go c.readH264AndPacket("./raw.h264")
		go c.readH264AndPacket1()
	}

	time.Sleep(time.Second * 120)
	if _, err = c.mediaClient.StopSession(ctx, &rpc.StopParam{SessionId: session.SessionId}, opts...); err != nil {
		panic(err)
	} else {
		fmt.Printf("StopSession %v\n", id)
	}
	time.Sleep(1 * time.Second)
	cancel()
	cancelRtp()
	p.Put(uint16(peerPort))
	p.Put(uint16(localPort))
	c.close()
	time.Sleep(3 * time.Second)

}

func (c *client) mockSendRtp(id string, localIpStr string, localPort int, remoteIpStr string, remotePort int, isAudio bool) (context.CancelFunc, error) {
	fmt.Printf("\nstream id:%v localip:%v localport:%v remoteip:%v remoteport:%v ", id, localIpStr, localPort, remoteIpStr, remotePort)
	localIp, _ := net.ResolveIPAddr("ip", localIpStr)
	remoteIp, _ := net.ResolveIPAddr("ip", remoteIpStr)
	tpLocal, err := rtp.NewTransportUDP(localIp, localPort, "")
	if err != nil {
		return nil, err
	}
	session := rtp.NewSession(tpLocal, tpLocal)
	strIndex, _ := session.NewSsrcStreamOut(&rtp.Address{
		IPAddr:   localIp.IP,
		DataPort: localPort,
		CtrlPort: 1 + localPort,
		Zone:     "",
	}, 0, 0)
	if isAudio {
		session.SsrcStreamOutForIndex(strIndex).SetProfile("PCMA", 8)
	} else {
		session.SsrcStreamOutForIndex(strIndex).SetProfile("H264", 123)
	}

	if _, err = session.AddRemote(&rtp.Address{
		IPAddr:   remoteIp.IP,
		DataPort: remotePort,
		CtrlPort: 1 + remotePort,
		Zone:     "",
	}); err != nil {
		return nil, err
	}
	if err = session.StartSession(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if isAudio {
		go func() {
			var pts uint32
			for {
				select {
				case payload, more := <-c.audioPacketChan:
					if !more {
						return
					}
					packet := session.NewDataPacket(pts)
					packet.SetPayloadType(8)
					packet.SetPayload(payload)
					session.WriteData(packet)
				case <-ctx.Done():
					session.CloseSession()
					return
				}
			}

		}()
	} else {
		go func() {
			for {
				select {
				case packet, more := <-c.h264PacketChan:
					if !more {
						return
					}
					pt, pts := packet.Payload, packet.Pts
					CPacketH264ToRtpAndSend(session, pt, uint32(pts), 123)

				case <-ctx.Done():
					session.CloseSession()
					return
				}
			}

		}()
	}

	return cancel, nil
}

func CPacketH264ToRtpAndSend(s *rtp.Session, annexbPayload []byte, pts uint32, payloadType uint8) {
	packetList := HCPacketListFromH264Mode(annexbPayload, pts, payloadType, 1200, false)

	packetList.Iterate(func(p *HCRtpPacketList) {
		payload, pt, pts1, mark := p.Payload, p.PayloadType, p.Pts, p.Marker
		if payload != nil {
			packet := s.NewDataPacket(pts1)
			packet.SetMarker(mark)
			packet.SetPayload(payload)
			packet.SetPayloadType(pt)

			if _, err := s.WriteData(packet); err != nil {
				fmt.Printf(" PacketH264ToRtpAndSend WriteData fail...\n")
			}

		}
	})
}

func (c *client) readH264AndPacket(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("open file fail %v\n", filePath)
		return
	}
	defer file.Close()

	buffer := make([]byte, 4096)
	framesPerSecond := 25
	frameDuration := time.Second / time.Duration(framesPerSecond)
	ticker := time.Tick(frameDuration)

	for range ticker {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Read file error:", err)
			}
			break
		}

		c.handleRawByte(buffer[:n])
	}
}

func (c *client) getAudioData(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("open file fail %v\n", filePath)
		return
	}
	defer file.Close()

	buffer := make([]byte, 160)
	ticker := time.Tick(20 * time.Millisecond)

	for range ticker {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Read file error:", err)
			}
			break
		}
		c.audioPacketChan <- buffer[:n]
	}
	return
}

func (c *client) updateMediaGraph(ctx context.Context, graphDesc string, sessionId string) {
	var opts []grpc.CallOption
	if _, err := c.mediaClient.ExecuteAction(ctx, &rpc.Action{
		SessionId: sessionId,
		Cmd:       "exec",
		CmdArg:    graphDesc,
	}, opts...); err != nil {
		panic(err)
	}
}

func (c *client) handleRawByte(data []byte) {

	pts := 0
	c.frame = append(c.frame, data...)

	for {
		startIndex1 := bytes.Index(c.frame, []byte{0x00, 0x00, 0x00, 0x01})
		startIndex2 := bytes.Index(c.frame, []byte{0x00, 0x00, 0x01})
		startIndex := -1

		if startIndex1 != -1 && startIndex2 != -1 {
			if startIndex1 < startIndex2 {
				startIndex = startIndex1
			} else {
				startIndex = startIndex2
			}
		} else if startIndex1 != -1 {
			startIndex = startIndex1
		} else if startIndex2 != -1 {
			startIndex = startIndex2
		}

		if startIndex == -1 {
			break
		}

		// 判断以哪种 start code 开头，并计算完整一帧的起始位置
		var frameStart int
		if startIndex1 != -1 && (startIndex2 == -1 || startIndex1 < startIndex2) {
			frameStart = startIndex + 4
		} else {
			frameStart = startIndex + 3
		}

		// 寻找下一个 start code 的位置
		nextStartIndex1 := bytes.Index(c.frame[frameStart:], []byte{0x00, 0x00, 0x00, 0x01})
		nextStartIndex2 := bytes.Index(c.frame[frameStart:], []byte{0x00, 0x00, 0x01})
		nextStartIndex := -1

		if nextStartIndex1 != -1 && nextStartIndex2 != -1 {
			if nextStartIndex1 < nextStartIndex2 {
				nextStartIndex = nextStartIndex1
			} else {
				nextStartIndex = nextStartIndex2
			}
		} else if nextStartIndex1 != -1 {
			nextStartIndex = nextStartIndex1
		} else if nextStartIndex2 != -1 {
			nextStartIndex = nextStartIndex2
		}

		if nextStartIndex == -1 {
			break
		}

		// 完整一帧的结束位置
		frameEnd := frameStart + nextStartIndex

		// 提取完整一帧数据
		avPacketData := c.frame[:frameEnd]
		pts += 3600

		packet := &MyH264Packet{
			Pts:     pts,
			Payload: avPacketData,
		}

		c.h264PacketChan <- packet
		c.frame = c.frame[frameEnd:]
	}

}

func (c *client) startVideoRecord(ctx context.Context, SessionId string) {
	desc := "[h264_file_sink] <-> 'play'"
	c.updateMediaGraph(ctx, desc, SessionId)
}

func (c *client) stopVideoRecord(ctx context.Context, SessionId string) {
	desc := "[h264_file_sink] <-> 'stop'"
	c.updateMediaGraph(ctx, desc, SessionId)
}

var (
	audioDataCache     []byte // 用于存储音频文件数据的全局缓存
	audioDataCacheLock sync.Mutex
	videoDataCache     []byte // 用于存储音频文件数据的全局缓存
	videoDataCacheLock sync.Mutex
)

func (c *client) getAudioData1() {
	ticker := time.Tick(20 * time.Millisecond)
	start := 0
	end := 160

	for range ticker {
		audioDataCacheLock.Lock()
		diff := len(audioDataCache) - end
		if diff < 0 {
			break
		}
		if diff >= 160 {
			data := audioDataCache[start:end]
			c.audioPacketChan <- data
			audioDataCacheLock.Unlock()
		} else {
			c.audioPacketChan <- audioDataCache[start:]
			audioDataCacheLock.Unlock()
			break
		}
		start += 160
		end += 160
	}
}

func (c *client) readH264AndPacket1() {
	framesPerSecond := 25
	frameDuration := time.Second / time.Duration(framesPerSecond)
	ticker := time.Tick(frameDuration)
	start := 0
	end := 4096

	for range ticker {
		videoDataCacheLock.Lock()
		diff := len(videoDataCache) - end
		if diff < 0 {
			break
		}
		if diff > 4096 {
			data := videoDataCache[start:end]
			c.handleRawByte(data)
			videoDataCacheLock.Unlock()
		} else {
			c.handleRawByte(videoDataCache[start:])
			videoDataCacheLock.Unlock()
			break
		}
		start += 4096
		end += 4096
	}
}

func InitAudioDataCache(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file failed: %v", err)
	}
	defer file.Close()

	// 读取文件数据并存储到缓存中
	var data []byte
	buffer := make([]byte, 4096)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("read file error: %v", err)
			}
			break
		}
		data = append(data, buffer[:n]...)
	}
	audioDataCache = data
	return nil
}

func InitVideoDataCache(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file failed: %v", err)
	}
	defer file.Close()

	// 读取文件数据并存储到缓存中
	var data []byte
	buffer := make([]byte, 4096)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("read file error: %v", err)
			}
			break
		}
		data = append(data, buffer[:n]...)
	}

	videoDataCache = data
	return nil
}
