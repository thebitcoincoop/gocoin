package main

import (
	"os"
	"fmt"
	"time"
	"bufio"
	"strings"
	"strconv"
	"runtime"
	"github.com/piotrnar/gocoin/btc"
)

type oneUiCmd struct {
	cmds []string // command name
	help string // a helf for this command
	sync bool  // shall be executed in the blochcina therad
	handler func(pars string) 
}

type oneUiReq struct {
	param string
	handler func(pars string) 
}


var uiCmds []*oneUiCmd


func newUi(cmds string, sync bool, hn func(string), help string) {
	cs := strings.Split(cmds, " ")
	if len(cs[0])>0 {
		var c = new(oneUiCmd)
		for i := range cs {
			c.cmds = append(c.cmds, cs[i])
		}
		c.sync = sync
		c.help = help
		c.handler = hn
		uiCmds = append(uiCmds, c)
	} else {
		panic("empty command string")
	}
}

func do_userif() {
	var prompt bool = true
	time.Sleep(5e8)
	for {
		if prompt {
			fmt.Print("> ")
		}
		li, _, _ := bufio.NewReader(os.Stdin).ReadLine()
		if len(li) > 0 {
			cmdpar := strings.SplitN(strings.Trim(string(li[:]), " \n\t\r"), " ", 2)
			cmd := cmdpar[0]
			param := ""
			if len(cmdpar)==2 {
				param = cmdpar[1]
			}
			prompt = true
			found := false
			for i := range uiCmds {
				for j := range uiCmds[i].cmds {
					if cmd==uiCmds[i].cmds[j] {
						found = true
						if uiCmds[i].sync {
							mutex.Lock()
							if busy!="" {
								print("now busy with ", busy)
							}
							mutex.Unlock()
							println("...")
							sta := time.Now().UnixNano()
							uiChannel <- oneUiReq{param:param, handler:uiCmds[i].handler}
							go func() {
								_ = <- uicmddone
								sto := time.Now().UnixNano()
								fmt.Printf("Ready in %.3fs\n", float64(sto-sta)/1e9)
								fmt.Print("> ")
							}()
							prompt = false
						} else {
							uiCmds[i].handler(param)
						}
					}
				}
			}
			if !found {
				fmt.Println("Unknown command:", cmd)
			}
		}
	}
}



func show_info(par string) {
	mutex.Lock()
	fmt.Printf("cachedBlocks:%d  pendingBlocks:%d/%d  receivedBlocks:%d\n", 
		len(cachedBlocks), len(pendingBlocks), len(pendingFifo), len(receivedBlocks))
	fmt.Printf("InvsIgn:%d  BlockDups:%d  InvsAsked:%d  NetMsgs:%d  UiMsgs:%d  Ticks:%d\n", 
		InvsIgnored, BlockDups, InvsAsked, NetMsgsCnt, UiMsgsCnt, TicksCnt)
	fmt.Println("LastBlock:", LastBlock.Height, LastBlock.BlockHash.String())
	if busy!="" {
		println("Currently busy with", busy)
	} else {
		println("Not busy")
	}
	mutex.Unlock()

	// memory usage:
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Println("HeapAlloc (memory used):", ms.HeapAlloc>>20, "MB")
}


func ui_beep(par string) {
	if par=="1" {
		beep = true
	} else if par=="0" {
		beep = false
	}
	fmt.Println("beep:", beep)
}


func ui_dbg(par string) {
	v, e := strconv.ParseUint(par, 10, 32)
	if e == nil {
		dbg = v
	}
	fmt.Println("dbg:", dbg)
}


func show_invs(par string) {
	mutex.Lock()
	fmt.Println(len(pendingBlocks), "pending invs")
	for _, v := range pendingBlocks {
		fmt.Println(v.String())
	}
	mutex.Unlock()
}


func show_cached(par string) {
	for _, v := range cachedBlocks {
		fmt.Printf(" * %s -> %s\n", v.Hash.String(), btc.NewUint256(v.Parent).String())
	}
}


func show_help(par string) {
	fmt.Println("There following commands are supported:")
	for i := range uiCmds {
		fmt.Print("   ")
		for j := range uiCmds[i].cmds {
			if j>0 {
				fmt.Print(", ")
			}
			fmt.Print(uiCmds[i].cmds[j])
		}
		fmt.Println(" -", uiCmds[i].help)
	}
	fmt.Println("All the commands are case sensitive.")
}


func init() {
	newUi("help h ?", false, show_help, "Shows this help")
	newUi("info i", false, show_info, "Shows general info")
	newUi("beep", false, ui_beep, "Control beep when a new block is received (use param 0 or 1)")
	newUi("dbg", false, ui_dbg, "Control debugs (use numeric parameter)")
	newUi("cach", false, show_cached, "Show blocks cached in memory")
	newUi("invs", false, show_invs, "Show pending block inv's (ones waiting for data)")
}

