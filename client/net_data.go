package main

import (
	"fmt"
	"time"
	"bytes"
	"sync/atomic"
	"encoding/binary"
	"github.com/piotrnar/gocoin/btc"
)


func (c *oneConnection) ProcessGetData(pl []byte) {
	//println(c.PeerAddr.Ip(), "getdata")
	b := bytes.NewReader(pl)
	cnt, e := btc.ReadVLen(b)
	if e != nil {
		println("ProcessGetData:", e.Error(), c.PeerAddr.Ip())
		return
	}
	for i:=0; i<int(cnt); i++ {
		var typ uint32
		var h [32]byte

		e = binary.Read(b, binary.LittleEndian, &typ)
		if e != nil {
			println("ProcessGetData:", e.Error(), c.PeerAddr.Ip())
			return
		}

		n, _ := b.Read(h[:])
		if n!=32 {
			println("ProcessGetData: pl too short", c.PeerAddr.Ip())
			return
		}

		CountSafe(fmt.Sprint("GetdataType",typ))
		if typ == 2 {
			uh := btc.NewUint256(h[:])
			bl, _, er := BlockChain.Blocks.BlockGet(uh)
			if er == nil {
				c.SendRawMsg("block", bl)
			} else {
				//println("block", uh.String(), er.Error())
			}
		} else if typ == 1 {
			// transaction
			uh := btc.NewUint256(h[:])
			tx_mutex.Lock()
			if tx, ok := TransactionsToSend[uh.Hash]; ok && tx.blocked==0 {
				tx.sentcnt++
				tx.lastsent = time.Now()
				tx_mutex.Unlock()
				c.SendRawMsg("tx", tx.data)
				if dbg > 0 {
					println("sent tx to", c.PeerAddr.Ip())
				}
			} else {
				tx_mutex.Unlock()
			}
		} else {
			if dbg>0 {
				println("getdata for type", typ, "not supported yet")
			}
		}
	}
}


func (c *oneConnection) GetBlockData(h []byte) {
	var b [1+4+32]byte
	b[0] = 1 // One inv
	b[1] = 2 // Block
	copy(b[5:37], h[:32])
	if dbg > 1 {
		println("GetBlockData", btc.NewUint256(h).String())
	}
	bh := btc.NewUint256(h)
	c.Mutex.Lock()
	c.GetBlockInProgress[bh.BIdx()] = &oneBlockDl{hash:bh, start:time.Now()}
	c.Mutex.Unlock()
	c.SendRawMsg("getdata", b[:])
}


// This function is called from a net conn thread
func netBlockReceived(conn *oneConnection, b []byte) {
	bl, e := btc.NewBlock(b)
	if e != nil {
		conn.DoS()
		println("NewBlock:", e.Error())
		return
	}

	idx := bl.Hash.BIdx()
	mutex_rcv.Lock()
	if rb, got := receivedBlocks[idx]; got {
		rb.cnt++
		mutex_rcv.Unlock()
		CountSafe("SameBlockReceived")
		return
	}
	orb := &oneReceivedBlock{Time:time.Now()}
	if bip, ok := conn.GetBlockInProgress[idx]; ok {
		orb.tmDownload = orb.Time.Sub(bip.start)
		conn.Mutex.Lock()
		delete(conn.GetBlockInProgress, idx)
		conn.Mutex.Unlock()
	} else {
		CountSafe("UnxpectedBlockRcvd")
	}
	receivedBlocks[idx] = orb
	mutex_rcv.Unlock()

	netBlocks <- &blockRcvd{conn:conn, bl:bl}
}


// Called from the blockchain thread
func HandleNetBlock(newbl *blockRcvd) {
	CountSafe("HandleNetBlock")
	bl := newbl.bl
	Busy("CheckBlock "+bl.Hash.String())
	e, dos, maybelater := BlockChain.CheckBlock(bl)
	if e != nil {
		if maybelater {
			addBlockToCache(bl, newbl.conn)
		} else {
			println(dos, e.Error())
			if dos {
				newbl.conn.DoS()
			}
		}
	} else {
		Busy("LocalAcceptBlock "+bl.Hash.String())
		e = LocalAcceptBlock(bl, newbl.conn)
		if e == nil {
			retryCachedBlocks = retry_cached_blocks()
		} else {
			println("AcceptBlock:", e.Error())
			newbl.conn.DoS()
		}
	}
}


// It goes through all the netowrk connections and checks
// ... how many of them have a given block download in progress
// Returns true if it's at the max already - don't want another one.
func blocksLimitReached(idx [btc.Uint256IdxLen]byte) (res bool) {
	var cnt uint32
	mutex_net.Lock()
	for _, v := range openCons {
		v.Mutex.Lock()
		_, ok := v.GetBlockInProgress[idx]
		v.Mutex.Unlock()
		if ok {
			if cnt+1 >= atomic.LoadUint32(&CFG.Net.MaxBlockAtOnce) {
				res = true
				break
			}
			cnt++
		}
	}
	mutex_net.Unlock()
	return
}

// Called from network threads
func blockWanted(h []byte) (yes bool) {
	idx := btc.NewUint256(h).BIdx()
	mutex_rcv.Lock()
	_, ok := receivedBlocks[idx]
	mutex_rcv.Unlock()
	if !ok {
		if atomic.LoadUint32(&CFG.Net.MaxBlockAtOnce)==0 || !blocksLimitReached(idx) {
			yes = true
			CountSafe("BlockWanted")
		} else {
			CountSafe("BlockInProgress")
		}
	} else {
		CountSafe("BlockUnwanted")
	}
	return
}
