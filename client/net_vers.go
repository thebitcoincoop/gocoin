package main

import (
	"time"
	"bytes"
	"errors"
	"encoding/binary"
	"github.com/piotrnar/gocoin/btc"
)


const (
	Version = 70001
	UserAgent = "/Gocoin:"+btc.SourcesTag+"/"
	Services = uint64(0x00000001)
)


func (c *oneConnection) SendVersion() {
	b := bytes.NewBuffer([]byte{})

	binary.Write(b, binary.LittleEndian, uint32(Version))
	binary.Write(b, binary.LittleEndian, uint64(Services))
	binary.Write(b, binary.LittleEndian, uint64(time.Now().Unix()))

	b.Write(c.PeerAddr.NetAddr.Bytes())
	if ExternalAddrLen()>0 {
		b.Write(BestExternalAddr())
	} else {
		b.Write(bytes.Repeat([]byte{0}, 26))
	}

	b.Write(nonce[:])

	b.WriteByte(byte(len(UserAgent)))
	b.Write([]byte(UserAgent))

	Last.mutex.Lock()
	binary.Write(b, binary.LittleEndian, uint32(Last.Block.Height))
	Last.mutex.Unlock()
	if !CFG.TXPool.Enabled {
		b.WriteByte(0)  // don't notify me about txs
	}

	c.SendRawMsg("version", b.Bytes())
}



func (c *oneConnection) HandleVersion(pl []byte) error {
	if len(pl) >= 80 /*Up to, includiong, the nonce */ {
		c.Mutex.Lock()
		c.node.version = binary.LittleEndian.Uint32(pl[0:4])
		if bytes.Equal(pl[72:80], nonce[:]) {
			c.Mutex.Unlock()
			return errors.New("Connecting to ourselves")
		}
		if c.node.version < MIN_PROTO_VERSION {
			c.Mutex.Unlock()
			return errors.New("Client version too low")
		}
		c.node.services = binary.LittleEndian.Uint64(pl[4:12])
		c.node.timestamp = binary.LittleEndian.Uint64(pl[12:20])
		c.Mutex.Unlock()
		if ValidIp4(pl[40:44]) {
			ExternalIpMutex.Lock()
			ExternalIp4[binary.BigEndian.Uint32(pl[40:44])]++
			ExternalIpMutex.Unlock()
		}
		if len(pl) >= 86 {
			c.Mutex.Lock()
			le, of := btc.VLen(pl[80:])
			of += 80
			c.node.agent = string(pl[of:of+le])
			of += le
			if len(pl) >= of+4 {
				c.node.height = binary.LittleEndian.Uint32(pl[of:of+4])
			}
			c.Mutex.Unlock()
		}
	} else {
		return errors.New("Version message too short")
	}
	c.SendRawMsg("verack", []byte{})
	return nil
}
