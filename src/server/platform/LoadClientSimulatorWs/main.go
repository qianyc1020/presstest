package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"server/platform/util"
	"strings"
	"sync"
	"sync/atomic"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//.\LoadClientSimulatorWs.exe -children= -httpaddr= -wsaddr= -mailboxs= -clients= -baseTest= -deltaClients= -deltaTime= -interval= -timeout=

//children 创建子进程数量
var children = flag.Int("children", 5, "")

//HTTPAddr HTTP请求token地址
var httpaddr = flag.String("httpaddr", "192.168.2.30:801", "")

//wsaddr Websocket登陆地址
var wsaddr = flag.String("wsaddr", "192.168.2.75:10000", "")

//dynamic 启用动态获取wsaddr，由HTTP返回网关ipaddr
var dynamic = flag.Int("dynamic", 0, "")

//numMailbox 单进程邮槽数，最好等于clients 5000
var numMailbox = flag.Int("mailboxs", 5000, "")

//totalClients 单进程登陆客户端总数
var totalClients = flag.Int("totalClients", 5000, "")

//numClients 单进程并发登陆客户端数<并发登陆>
var numClients = flag.Int("numClients", 1000, "")

//numClients2 单进程并发进房间客户端数<并发进房间>
var numClients2 = flag.Int("numClients2", 100, "")

//numClients3 单进程并发投注客户端数<并发投注>
var numClients3 = flag.Int("numClients3", 2000, "")

//BaseAccount 测试起始账号
var baseAccount = flag.Int64("baseTest", 12345, "")

//deltaClients 间隔连接数检查时间戳
var deltaClients = flag.Int("deltaClients", 500, "")

//deltaTime 间隔毫秒数检查连接数
var deltaTime = flag.Int("deltaTime", 8000, "")

//heartbeat 心跳间隔毫秒数
var heartbeat = flag.Int("interval", 8000, "")

//timeout 心跳超时清理毫秒数 timeout>interval
var timeout = flag.Int("timeout", 10000, "")

//subGameID 测试子游戏，游戏类型
var subGameID = flag.Int("gameID", 210, "")

//subRoomID 测试子游戏，房间号
var subRoomID = flag.Int("roomID", 2101, "")

//tokenprefix 测试token，免http登陆
var tokenprefix = flag.String("prefix", "test_new0_", "")
var tokenstart = flag.Int("tokenstart", 0, "")
var tokenend = flag.Int("tokenend", 99999, "")

//gProcMap 子进程
var gProcMap map[int]*os.Process
var gLock *sync.Mutex
var wg sync.WaitGroup
var gChildren int64

//onInput 输入命令行参数 'q'退出
func onInput(str string) int {
	switch str {
	case "c":
		{
			util.ClearScreen[runtime.GOOS]()
			return 0
		}
	case "q":
		{
			killAllChild()
			return -1
		}
	}
	return 0
}

//killAllChild 杀死所有子进程
func killAllChild() {
	gLock.Lock()
	for _, p := range gProcMap {
		err := p.Kill()
		if err != nil {
			log.Println(err)
		}
	}
	gProcMap = map[int]*os.Process{}
	gLock.Unlock()
}

//killChild 杀死子进程
func killChild(p *os.Process) {
	err := p.Kill()
	if err != nil {
		log.Println(err)
	}
	gLock.Lock()
	if _, ok := gProcMap[p.Pid]; ok {
		delete(gProcMap, p.Pid)
	}
	gLock.Unlock()
}

//waitChild 等待子进程退出
func waitChild(p *os.Process) {
	sta, err := p.Wait()
	if err != nil {
		log.Println(err)
	} else if sta.Success() {
		//exit status 0
	} else {
		//exit status 1
	}
	gLock.Lock()
	if _, ok := gProcMap[p.Pid]; ok {
		delete(gProcMap, p.Pid)
	}
	gLock.Unlock()
	atomic.AddInt64(&gChildren, -1)
	wg.Done()
}

func loadConf() bool {
	c := readini("conf.ini")
	if c == nil {
		return false
	}
	if c.flag == 1 {
		//解析命令行解析
		flag.Parse()
	} else {
		//从配置读取参数
		*children = c.children
		*httpaddr = c.httpaddr
		*wsaddr = c.wsaddr
		*dynamic = c.dynamic
		*numMailbox = c.numMailbox
		*totalClients = c.totalClients
		*numClients = c.numClients
		*numClients2 = c.numClients2
		*numClients3 = c.numClients3
		*baseAccount = c.baseAccount
		*deltaClients = c.deltaClients
		*deltaTime = c.deltaTime
		*heartbeat = c.heartbeat
		*timeout = c.timeout
		*subGameID = c.subgameID
		*subRoomID = c.subroomID
		*tokenprefix = c.tokenprefix
		*tokenstart = c.tokenstart
		*tokenend = c.tokenend
	}
	return true
}
func main() {
	if !loadConf() {
		return
	}
	gLock = &sync.Mutex{}
	gProcMap = map[int]*os.Process{}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	var execname, execStr string
	if runtime.GOOS == "linux" {
		dir += "/../ClientSimulatorWs2/"
		execname = "ClientSimulatorWs2"
		execStr = "./" + execname
	} else if runtime.GOOS == "windows" {
		dir += "\\..\\ClientSimulatorWs2\\"
		execname = "ClientSimulatorWs2.exe"
		execStr = execname
	}
	f, err := exec.LookPath(dir + execname)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		util.ReadConsole(onInput)
	}()
	//启动客户端进程数*children
	for i := 0; i < *children; i++ {
		cmdLine := fmt.Sprintf("%s -httpaddr=%s -dynamic=%d -wsaddr=%s -mailboxs=%d -totalClients=%d -numClients=%d -numClients2=%d -numClients3=%d -baseTest=%d -deltaClients=%d -deltaTime=%d -interval=%d -timeout=%d -gameID=%d -roomID=%d -prefix=%s -tokenstart=%d -tokenend=%d",
			execStr,
			*httpaddr, *dynamic, *wsaddr, *numMailbox, *totalClients, *numClients, *numClients2, *numClients3, *baseAccount+int64(*totalClients)*int64(i), *deltaClients, *deltaTime, *heartbeat, *timeout, *subGameID, *subRoomID,
			*tokenprefix, *tokenstart+int(*totalClients)*int(i), *tokenend)
		args := strings.Split(cmdLine, " ")
		attr := &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		}
		p, err := os.StartProcess(f, args, attr)
		if err != nil {
			log.Println("StartProcess: ", err)
			continue
		}
		atomic.AddInt64(&gChildren, 1)
		wg.Add(1)
		go waitChild(p)
		gProcMap[p.Pid] = p
	}
	log.Printf("Children = Succ[%03d]\n", atomic.LoadInt64(&gChildren))
	wg.Wait()
	log.Println("exit...")
}
