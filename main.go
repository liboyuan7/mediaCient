package main

import (
	"encoding/json"
	"fmt"
	"github.com/streamFunc/mediaClient/client"
	"github.com/streamFunc/mediaClient/port"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

/*var (
	serverAddr  = flag.String("server_addr", "127.0.0.1:5678", "The server address in the format of host:port")
	concurrency = flag.Int("concurrency", 4, "The number of concurrent requests audio and video")
	numRequests = flag.Int("num_requests", 12, "The total number of requests audio and video to be made")
	runTime = flag.Int("run_time", 12, "The running time for every session")
)*/

func main() {
	parseConfigJson()

	//flag.Parse()

	ip, gPort, err := net.SplitHostPort(globalConfig.GrpcAddr)
	client.GrpcIp = ip
	client.GrpcPort, _ = strconv.Atoi(gPort)

	fmt.Printf("start grpc %v port %v\n", ip, gPort)

	if err != nil {
		fmt.Println("Error parsing server address:", err)
		return
	}

	client.InitAudioDataCache("./audio.pcm")
	client.InitVideoDataCache("./raw.h264")

	portRequest := port.NewPortPool()
	portRequest.Init(uint16(globalConfig.StartPort), uint16(globalConfig.EndPort))

	// Perform the benchmark.
	start := time.Now()
	runBenchmark(globalConfig.ConCurrency, portRequest)
	elapsed := time.Since(start)

	fmt.Printf("Total time taken: %s\n", elapsed)
}

/*func runBenchmark(concurrency, numRequests int, p *port.MyPortPool) {
	var wg sync.WaitGroup
	requestsPerWorker := numRequests / concurrency
	var index int32
	var flag int32
	requestsPerSecond := 500
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				currentIndex := atomic.AddInt32(&index, 1)
				id := "new_session_" + strconv.Itoa(int(currentIndex))

				if globalConfig.Mode == 0 {
					client.StartSessionCall(p, id, true, globalConfig.AudioGraph, globalConfig.RunTime)
				} else if globalConfig.Mode == 1 {
					client.StartSessionCall(p, id, false, globalConfig.VideoGraph, globalConfig.RunTime)
				} else if globalConfig.Mode == 2 {
					// 原子操作，确保并发安全的读取和修改 flag
					isAudio := atomic.LoadInt32(&flag) == 1
					atomic.StoreInt32(&flag, 1-atomic.LoadInt32(&flag))
					if isAudio {
						//gd := " [rtp_src] ->[rtp_pcm_alaw] -> [asrt:audio_transcode from_codec=pcm_alaw from_sample_rate=8000 to_codec=pcm_s16le to_sample_rate=16000] ->
						//[evs_encoder channels=1 sample_rate=16000 encoderFormat=WB bitRate=24400 codec_name=pcma] -> [evs_decoder sample_rate=16000 bitRate=24400] ->[audio_rtp payloadType=106] -> [rtp_sink];\n"
						client.StartSessionCall(p, id, true, globalConfig.AudioGraph, globalConfig.RunTime)
					} else {
						client.StartSessionCall(p, id, false, globalConfig.VideoGraph, globalConfig.RunTime)
					}
				} else {
					fmt.Printf("not support mode,error\n")
				}
				time.Sleep(time.Second / time.Duration(requestsPerSecond))
			}
		}(i)
		//time.Sleep(time.Millisecond * 1)
	}

	// Wait for all workers to finish.
	wg.Wait()
}*/

func runBenchmark(concurrency int, p *port.MyPortPool) {
	var index int32
	var flag int32
	requestsPerSecond := globalConfig.RequestsPerSecond
	duration := globalConfig.Duration
	gameOver := false

	// 创建初始的并发数
	for i := 0; i < concurrency; i++ {
		go createSession(p, &index, &flag)
	}

	if requestsPerSecond >= 0 {
		ticker := time.NewTicker(time.Second * time.Duration(globalConfig.Interval))
		defer ticker.Stop()

		timeout := time.After(time.Duration(duration) * time.Second)
		for {
			select {
			case <-ticker.C:
				for i := 0; i < requestsPerSecond; i++ {
					go createSession(p, &index, &flag)
				}
			case <-timeout:
				// 完成持续时间
				gameOver = true
				return
			}
		}
	}

	for {
		if gameOver {
			break
		}
		time.Sleep(time.Second)
	}

}

func createSession(p *port.MyPortPool, index, flag *int32) {

	currentIndex := atomic.AddInt32(index, 1)
	id := "new_session_" + strconv.Itoa(int(currentIndex))

	if globalConfig.Mode == 0 {
		client.StartSessionCall(p, id, true, globalConfig.AudioGraph, globalConfig.RtpRunTime)
	} else if globalConfig.Mode == 1 {
		client.StartSessionCall(p, id, false, globalConfig.VideoGraph, globalConfig.RtpRunTime)
	} else if globalConfig.Mode == 2 {
		isAudio := atomic.LoadInt32(flag) == 1
		atomic.StoreInt32(flag, 1-atomic.LoadInt32(flag))
		if isAudio {
			client.StartSessionCall(p, id, true, globalConfig.AudioGraph, globalConfig.RtpRunTime)
		} else {
			client.StartSessionCall(p, id, false, globalConfig.VideoGraph, globalConfig.RtpRunTime)
		}
	} else {
		fmt.Printf("not support mode,error\n")
	}
}

// ClientConfig 定义一个结构体来表示 JSON 文件中的数据结构
type ClientConfig struct {
	GrpcAddr          string `json:"grpcAddr"`
	StartPort         int    `json:"startPort"`
	EndPort           int    `json:"endPort"`
	ConCurrency       int    `json:"conCurrency"`
	RequestsPerSecond int    `json:"requestsPerSecond"`
	Interval          int    `json:"interval"`
	Duration          int    `json:"duration"`
	RtpRunTime        int    `json:"rtpRunTime"`
	AudioGraph        string `json:"audioGraph"`
	VideoGraph        string `json:"videoGraph"`
	Mode              int    `json:"mode"`
}

// 创建一个变量来存储解析后的 JSON 数据
var globalConfig ClientConfig

func parseConfigJson() {
	file, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error opening JSON file:", err)
		return
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&globalConfig)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	fmt.Println("GrpcAddr:", globalConfig.GrpcAddr)
	fmt.Println("StartPort:", globalConfig.StartPort)
	fmt.Println("EndPort:", globalConfig.EndPort)
	fmt.Println("ConCurrency:", globalConfig.ConCurrency)
	fmt.Println("requestsPerSecond:", globalConfig.RequestsPerSecond)
	fmt.Println("Interval:", globalConfig.Interval)
	fmt.Println("Duration:", globalConfig.Duration)
	fmt.Println("runTime:", globalConfig.RtpRunTime)
	fmt.Println("audioGraph:", globalConfig.AudioGraph)
	fmt.Println("videoGraph:", globalConfig.VideoGraph)
	fmt.Println("mode:", globalConfig.Mode)
}
