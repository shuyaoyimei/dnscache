package main

import (
	"net"
	"time"

	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/miekg/dns"
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
	resolver        *Resolver
	cache, negCache Cache
}

func NewHandler() *GODNSHandler {

	var (
		resolver        *Resolver
		cache, negCache Cache
	)

	resolver = &Resolver{}

	cache = &MemoryCache{
		Backend:  make(map[string]Mesg, 1024),
		Expire:   time.Duration(3600) * time.Second,
		Maxcount: 1024,
	}
	negCache = &MemoryCache{
		Backend:  make(map[string]Mesg),
		Expire:   time.Duration(30) * time.Second / 2,
		Maxcount: 1024,
	}

	return &GODNSHandler{resolver, cache, negCache}
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
	fmt.Println("%s lookup　%s", remote, Q.String())

	IPQuery := h.isIPQuery(q)

	// Only query cache when qtype == 'A'|'AAAA' , qclass == 'IN'
	hasher := md5.New()
	hasher.Write([]byte(Q.String()))
	key := hex.EncodeToString(hasher.Sum(nil))
	if IPQuery > 0 {
		mesg, err := h.cache.Get(key)
		if err != nil {
			if mesg, err = h.negCache.Get(key); err != nil {
				fmt.Println("%s didn't hit cache", Q.String())
			} else {
				fmt.Println("%s hit negative cache", Q.String())
				dns.HandleFailed(w, req)
				return
			}
		} else {
			fmt.Println("%s hit cache", Q.String())
			// we need this copy against concurrent modification of Id
			msg := *mesg
			msg.Id = req.Id
			w.WriteMsg(&msg)
			return
		}
	}

	mesg, err := h.resolver.Lookup(Net, req)

	if err != nil {
		fmt.Println("Resolve query error %s", err)
		dns.HandleFailed(w, req)

		// cache the failure, too!
		if err = h.negCache.Set(key, nil); err != nil {
			fmt.Println("Set %s negative cache failed: %v", Q.String(), err)
		}
		return
	}

	w.WriteMsg(mesg)

	if IPQuery > 0 && len(mesg.Answer) > 0 {
		err = h.cache.Set(key, mesg)
		if err != nil {
			fmt.Println("Set %s cache failed: %s", Q.String(), err.Error())
		}
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
