package main

import (
	"encoding/json"
	"fmt"
	"github.com/streamFunc/mediaClient/client"
	"github.com/streamFunc/mediaClient/port"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

/*var (
	serverAddr  = flag.String("server_addr", "127.0.0.1:5678", "The server address in the format of host:port")
	concurrency = flag.Int("concurrency", 4, "The number of concurrent requests audio and video")
	numRequests = flag.Int("num_requests", 12, "The total number of requests audio and video to be made")
)*/

func main() {
	parseGraph()

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
	portRequest.Init(20000, 30000)

	// Perform the benchmark.
	start := time.Now()
	runBenchmark(globalConfig.ConCurrency, globalConfig.NumRequests, portRequest)
	elapsed := time.Since(start)

	fmt.Printf("Total time taken: %s\n", elapsed)
	fmt.Printf("Requests per second: %.2f\n", float64(globalConfig.NumRequests)/elapsed.Seconds())
}

func runBenchmark(concurrency, numRequests int, p *port.MyPortPool) {
	var wg sync.WaitGroup
	requestsPerWorker := numRequests / concurrency
	var index int32
	var flag int32
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				currentIndex := atomic.AddInt32(&index, 1)
				id := "new_session_" + strconv.Itoa(int(currentIndex))

				// 原子操作，确保并发安全的读取和修改 flag
				isAudio := atomic.LoadInt32(&flag) == 1
				atomic.StoreInt32(&flag, 1-atomic.LoadInt32(&flag))
				if isAudio {
					//gd := " [rtp_src] ->[rtp_pcm_alaw] -> [asrt:audio_transcode from_codec=pcm_alaw from_sample_rate=8000 to_codec=pcm_s16le to_sample_rate=16000] ->
					//[evs_encoder channels=1 sample_rate=16000 encoderFormat=WB bitRate=24400 codec_name=pcma] -> [evs_decoder sample_rate=16000 bitRate=24400] ->[audio_rtp payloadType=106] -> [rtp_sink];\n"
					client.StartSessionCall(p, id, true, globalConfig.AudioGraph)
				} else {
					client.StartSessionCall(p, id, false, globalConfig.VideoGraph)
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Wait for all workers to finish.
	wg.Wait()
}

// ClientConfig 定义一个结构体来表示 JSON 文件中的数据结构
type ClientConfig struct {
	GrpcAddr    string `json:"grpcAddr"`
	ConCurrency int    `json:"conCurrency"`
	NumRequests int    `json:"numRequests"`
	AudioGraph  string `json:"audioGraph"`
	VideoGraph  string `json:"videoGraph"`
}

// 创建一个变量来存储解析后的 JSON 数据
var globalConfig ClientConfig

func parseGraph() {
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
	fmt.Println("ConCurrency:", globalConfig.ConCurrency)
	fmt.Println("NumRequests:", globalConfig.NumRequests)
	fmt.Println("audioGraph:", globalConfig.AudioGraph)
	fmt.Println("videoGraph:", globalConfig.VideoGraph)
}
