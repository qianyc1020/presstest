package main

//
// Created by andy_ro@qq.com
// 			4/9/2019
//

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"server/platform/util"
	"sync"
	"sync/atomic"
	"time"
)

//////////////////////////////////////////////////////////////////////////////////////////////////////////
//.\ClientSimulatorWs2.exe -httpaddr= -wsaddr= -mailboxs= -totalClients=%d -numClients=%d -numClients2=%d -numClients3=%d -baseTest= -deltaClients= -deltaTime= -interval= -timeout=

//HTTPAddr HTTP请求token地址
var httpaddr = flag.String("httpaddr", "192.168.2.214", "")

//wsaddr Websocket登陆地址
var wsaddr = flag.String("wsaddr", "192.168.2.211:10000", "")

//dynamic 启用动态获取wsaddr，由HTTP返回网关ipaddr
var dynamic = flag.Int("dynamic", 1, "")

//numMailbox 单进程邮槽数，最好等于clients 5000
var numMailbox = flag.Int("mailboxs", 100, "")

//totalClient 单进程登陆客户端总数
var totalClients = flag.Int("totalClients", 1, "")

//numClients 单进程并发登陆客户端数<并发登陆>
var numClients = flag.Int("numClients", 1, "")

//numClients2 单进程并发进房间客户端数<并发进房间>
var numClients2 = flag.Int("numClients2", 1, "")

//numClients3 单进程并发投注客户端数<并发投注>
var numClients3 = flag.Int("numClients3", 1, "")

//BaseAccount 测试起始账号
var baseAccount = flag.Int64("baseTest", 200, "")

//deltaClients 间隔连接数检查时间戳
var deltaClients = flag.Int("deltaClients", 500, "")

//deltaTime 间隔毫秒数检查连接数
var deltaTime = flag.Int("deltaTime", 8000, "")

//heartbeat 心跳间隔毫秒数
var heartbeat = flag.Int("interval", 5000, "")

//timeout 心跳超时清理毫秒数 timeout>interval
var timeout = flag.Int("timeout", 30000, "")

//subGameID 测试子游戏，游戏类型
var subGameID = flag.Int("gameID", 720, "")

//subRoomID 测试子游戏，房间号
var subRoomID = flag.Int("roomID", 7201, "")

//tokenprefix 测试token，免http登陆
var tokenprefix = flag.String("prefix", "test_new2_", "")
var tokenstart = flag.Int("tokenstart", 100, "")
var tokenend = flag.Int("tokenend", 99999, "")

//timestart 起始时间戳
var timestart, timestart2 Timestamp

//curConn 当前连接数
var curConn int64

//timenow 当前时间戳
var timenow Timestamp

//
const (
	StepNil = iota
	StepLogin
	StepEnter
	StepAll
)

var gStep = StepNil

//gSemLogin 登陆并发访问控制
var gSemLogin *util.Semaphore

//gSemEnter 进房间并发访问控制
var gSemEnter *util.Semaphore

//gSemJetton 投注并发访问控制
var gSemJetton *util.FreeSemaphore

//onInput 输入命令行参数 'q'退出 'c'清屏
func onInput(str string) int {
	switch str {
	case "q":
		{
			gSessMgr.CloseAll()
			gMailbox.Stop()
			return -1
		}
	case "c":
		{
			util.ClearScreen[runtime.GOOS]()
			return 0
		}
	}
	return 0
}

//
func StartDaemon() {
	go func() {
		for StepAll != gStep {
			time.Sleep(100)
			if StepLogin == gStep &&
				atomic.LoadInt64(&gClients) >= int64(*totalClients) {
				gStep = StepEnter
				*totalClients = int(gClientsSucc)
				gClients = 0
				gClientsSucc = 0
				gClientsFailed = 0
				elapsed = 0
				//发起并发进房间请求
				ParallEnterRoomRequest()
			} else {
				if StepNil == gStep {
					gStep = StepLogin
					//发起并发连接/登陆请求
					ParallLoginRequest()
				}
			}
		}
	}()
	//控制台命令行输入 按'q'退出 'c'清屏q
	go util.ReadConsole(onInput)
}

//gClients 登陆总数
var gClients = int64(0)

//gClientsSucc 登陆成功总数
var gClientsSucc = int64(0)

//gClientsFailed 登陆失败总数
var gClientsFailed = int64(0)

//gSuccPeers 登陆成功会话
var gSuccPeers = []int64{}
var gL = &sync.Mutex{}
var elapsed int32
var gEnters = int64(0)
var gEntersSucc = int64(0)
var gEntersFailed = int64(0)

//
func AddSuccPeer(sesID int64) {
	gL.Lock()
	gSuccPeers = append(gSuccPeers, sesID)
	gL.Unlock()
}

//
func PopPeer() (id int64) {
	gL.Lock()
	if len(gSuccPeers) > 0 {
		id = gSuccPeers[0]
		gSuccPeers = gSuccPeers[1:]
	}
	gL.Unlock()
	return
}

//
func main() {
	// util.TestSemaphore()
	// util.ReadConsole(util.OnInputTestSemaphore)
	defer func() {
		if r := recover(); r != nil {
			log.Println(debug.Stack())
		}
	}()
	log.Printf("--- *** PID[%07d] %v\n", os.Getpid(), os.Args)
	//解析命令行
	flag.Parse()
	//worker工厂
	smain := NewSentryCreator()
	//启动10000个邮槽
	t1 := TimeNowMilliSec()
	gMailbox.Start(smain, *numMailbox, (*timeout)/1000+10)
	t2 := TimeNowMilliSec()
	log.Printf("--- *** PID[%07d] gMailbox.Start = [%03d] elapsed = %dms\n", os.Getpid(), *numMailbox, TimeDiff(t2, t1))
	//并发登陆控制
	gSemLogin = util.NewSemaphore(int64(*numClients))
	//并发进房间控制
	gSemEnter = util.NewSemaphore(int64(*numClients2))
	//并发投注控制
	gSemJetton = util.NewFreeSemaphore(int64(*numClients3))
	//开始运行
	StartDaemon()
	//等待退出
	gMailbox.Wait()
	gSessMgr.Wait()
	log.Printf("--- *** PID[%07d] exit...", os.Getpid())
}
