package prober

import (
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (p *Prober) sender() {
	defer p.rawConn.Close()

	p.desynchronizeStartTime()
	err := p.setLocalAddr()
	if err != nil {
		log.Errorf("unable to set local address: %v", err)
	}
	seq := uint64(0)
	pr := probe{}
	t := time.NewTicker(time.Second / time.Duration(p.cfg.PPS))

	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
		}

		pr.SequenceNumber = seq
		pr.TimeStamp = time.Now().UnixNano()
		pkt, err := p.craftPacket(&pr)
		if err != nil {
			log.Errorf("Unable to craft packet: %v", err)
			continue
		}

		p.transitProbes.add(&pr)

		tsAligned := pr.TimeStamp - (pr.TimeStamp % (int64(p.cfg.MeasurementLengthMS) * int64(time.Millisecond)))
		p.measurements.AddSent(tsAligned)

		srcAddr := p.getSrcAddr(seq)
		dstAddr := p.cfg.Hops[0].getAddr(seq)
		err = p.sendPacket(pkt, srcAddr, dstAddr)
		if err != nil {
			log.Errorf("Unable to send packet: %v", err)
			err = p.transitProbes.remove(pr.SequenceNumber)
			if err != nil {
				log.Errorf("unable to remove probe for probe number %v: %v", pr.SequenceNumber, err)
			}
			continue
		}

		atomic.AddUint64(&p.probesSent, 1)
		seq++
	}
}

func (p *Prober) sendPacket(payload []byte, src net.IP, dst net.IP) error {
	options := writeOptions{
		src:      src,
		dst:      dst,
		tos:      int64(p.cfg.TOS.Value),
		ttl:      ttl,
		protocol: unix.IPPROTO_GRE,
	}

	if err := p.rawConn.WriteTo(payload, options); err != nil {
		return fmt.Errorf("Unable to send packet: %v", err)
	}

	return nil
}

func (p *Prober) desynchronizeStartTime() {
	time.Sleep(time.Duration(random(int64(p.cfg.TimeoutMS))) * time.Microsecond)
}

func random(max int64) int {
	return rand.Intn(int(max))
}
