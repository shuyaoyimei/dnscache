package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/golang/groupcache/lru"
	"github.com/miekg/dns"
	"net"
)

const (
	notIPQuery = 0
	_IP4Query  = 4
	_IP6Query  = 6
)

type Question struct {
	qname  string
	qtype  string
	qclass string
}

func (q *Question) String() string {
	return q.qname + " " + q.qclass + " " + q.qtype
}

type GODNSHandler struct {
	resolver *Resolver
	Cache    *lru.Cache
}

func NewHandler() *GODNSHandler {

	var (
		resolver *Resolver
		Cache    *lru.Cache
	)
	resolver = &Resolver{}
	Cache = lru.New(8192)
	return &GODNSHandler{resolver, Cache}
}

func (h *GODNSHandler) do(Net string, w dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	Q := Question{UnFqdn(q.Name), dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass]}

	var remote net.IP
	if Net == "tcp" {
		remote = w.RemoteAddr().(*net.TCPAddr).IP
	} else {
		remote = w.RemoteAddr().(*net.UDPAddr).IP
	}
	fmt.Println("DNS Lookup ", remote, Q.String())

	IPQuery := h.isIPQuery(q)

	// Only query cache when qtype == 'A'|'AAAA' , qclass == 'IN'
	hasher := md5.New()
	hasher.Write([]byte(Q.String()))
	key := hex.EncodeToString(hasher.Sum(nil))
	if IPQuery > 0 {
		mesg, ok := h.Cache.Get(key)
		if ok == true {
			fmt.Println("Hit cache", Q.String())
			msg := mesg.(*dns.Msg)
			msg.Id = req.Id
			w.WriteMsg(msg)
			return
		}
	}

	mesg, err := h.resolver.Lookup(Net, req)

	if err != nil {
		fmt.Println("Resolve query error ", err)
		dns.HandleFailed(w, req)
		return
	}

	w.WriteMsg(mesg)

	if IPQuery > 0 && len(mesg.Answer) > 0 {
		h.Cache.Add(key, mesg)
		fmt.Println("Insert %s into cache", Q.String())
	}
}

func (h *GODNSHandler) DoTCP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("tcp", w, req)
}

func (h *GODNSHandler) DoUDP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("udp", w, req)
}

func (h *GODNSHandler) isIPQuery(q dns.Question) int {
	if q.Qclass != dns.ClassINET {
		return notIPQuery
	}

	switch q.Qtype {
	case dns.TypeA:
		return _IP4Query
	case dns.TypeAAAA:
		return _IP6Query
	default:
		return notIPQuery
	}
}

func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
